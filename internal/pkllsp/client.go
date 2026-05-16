package pkllsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc
	done   chan struct{}
	logger *slog.Logger

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan response
	opened  map[string]int32
	diags   map[string]diagnosticState
	closed  bool
}

type diagnosticState struct {
	version     int32
	diagnostics []lspDiagnostic
}

type response struct {
	result json.RawMessage
	err    error
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type lspPosition struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspDiagnostic struct {
	Range    lspRange        `json:"range"`
	Severity int32           `json:"severity,omitempty"`
	Code     json.RawMessage `json:"code,omitempty"`
	Message  string          `json:"message"`
}

type completionItem struct {
	Label      string          `json:"label"`
	Kind       int32           `json:"kind,omitempty"`
	Detail     string          `json:"detail,omitempty"`
	InsertText string          `json:"insertText,omitempty"`
	TextEdit   json.RawMessage `json:"textEdit,omitempty"`
}

type hoverResult struct {
	Contents json.RawMessage `json:"contents"`
}

type location struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

func startClient(ctx context.Context, cfg Config) (*client, error) {
	bin, err := resolveBinary(cfg.BinaryPath)
	if err != nil {
		return nil, err
	}

	procCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(procCtx, bin)
	cmd.Env = os.Environ()
	if cfg.SwitchyardNamespaceDir != "" {
		cmd.Env = append(cmd.Env, "PKL_LSP_NAMESPACES=switchyard="+cfg.SwitchyardNamespaceDir)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("pkl-lsp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("pkl-lsp stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("pkl-lsp stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start pkl-lsp %q: %w", bin, err)
	}

	c := &client{
		cmd:     cmd,
		stdin:   stdin,
		cancel:  cancel,
		done:    make(chan struct{}),
		logger:  cfg.Logger,
		nextID:  1,
		pending: map[int64]chan response{},
		opened:  map[string]int32{},
		diags:   map[string]diagnosticState{},
	}
	go c.readLoop(procCtx, stdout)
	go c.stderrLoop(stderr)
	go c.waitLoop()

	if err := c.initialize(ctx, cfg); err != nil {
		closeCtx, closeCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer closeCancel()
		if closeErr := c.close(closeCtx); closeErr != nil && cfg.Logger != nil {
			cfg.Logger.Debug("pkl-lsp close after initialize failure", "err", closeErr)
		}
		return nil, err
	}
	return c, nil
}

func (c *client) initialize(ctx context.Context, cfg Config) error {
	rootURI := ""
	if cfg.ConfigDir != "" {
		rootURI = fileURI(cfg.ConfigDir)
	}
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"window": map[string]any{"workDoneProgress": false},
		},
		"initializationOptions": map[string]any{
			"namespaces": map[string]string{
				"switchyard": cfg.SwitchyardNamespaceDir,
			},
		},
	}
	if _, err := c.request(ctx, "initialize", params); err != nil {
		return fmt.Errorf("initialize pkl-lsp: %w", err)
	}
	return c.notify("initialized", map[string]any{})
}

func (c *client) close(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		select {
		case <-c.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.closed = true
	for id, ch := range c.pending {
		delete(c.pending, id)
		ch <- response{err: errors.New("pkl-lsp stopped")}
	}
	c.mu.Unlock()
	_ = c.stdin.Close()
	c.cancel()
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *client) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *client) waitLoop() {
	defer close(c.done)
	err := c.cmd.Wait()
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	for id, ch := range c.pending {
		delete(c.pending, id)
		ch <- response{err: fmt.Errorf("pkl-lsp exited: %w", err)}
	}
	c.mu.Unlock()
}

func (c *client) stderrLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if c.logger != nil {
			c.logger.Debug("pkl-lsp", "msg", sc.Text())
		}
	}
}

func (c *client) readLoop(ctx context.Context, r io.Reader) {
	br := bufio.NewReader(r)
	for {
		body, err := readFramedMessage(br)
		if err != nil {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			if closeErr := c.close(closeCtx); closeErr != nil && c.logger != nil {
				c.logger.Debug("pkl-lsp close after read failure", "err", closeErr)
			}
			cancel()
			return
		}
		c.handleMessage(body)
	}
}

func readFramedMessage(br *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, errors.New("missing Content-Length")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(br, body)
	return body, err
}

func (c *client) handleMessage(body []byte) {
	var msg struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return
	}
	if len(msg.ID) > 0 {
		var id int64
		if err := json.Unmarshal(msg.ID, &id); err != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[id]
		delete(c.pending, id)
		c.mu.Unlock()
		if ch == nil {
			return
		}
		if msg.Error != nil {
			ch <- response{err: fmt.Errorf("pkl-lsp rpc %d: %s", msg.Error.Code, msg.Error.Message)}
			return
		}
		ch <- response{result: msg.Result}
		return
	}
	if msg.Method == "textDocument/publishDiagnostics" {
		var params struct {
			URI         string          `json:"uri"`
			Version     *int32          `json:"version"`
			Diagnostics []lspDiagnostic `json:"diagnostics"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return
		}
		version := int32(0)
		if params.Version != nil {
			version = *params.Version
		}
		c.mu.Lock()
		c.diags[params.URI] = diagnosticState{version: version, diagnostics: params.Diagnostics}
		c.mu.Unlock()
	}
}

func (c *client) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id, ch, err := c.reserveRequest()
	if err != nil {
		return nil, err
	}
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	if err := c.writeJSON(msg); err != nil {
		c.dropRequest(id)
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.dropRequest(id)
		return nil, ctx.Err()
	case resp := <-ch:
		return resp.result, resp.err
	}
}

func (c *client) reserveRequest() (int64, chan response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, nil, errors.New("pkl-lsp is not running")
	}
	id := c.nextID
	c.nextID++
	ch := make(chan response, 1)
	c.pending[id] = ch
	return id, ch, nil
}

func (c *client) dropRequest(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *client) notify(method string, params any) error {
	return c.writeJSON(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *client) writeJSON(msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}

func (c *client) syncDocument(filePath, source string) (string, int32, error) {
	uri := fileURI(filePath)
	c.mu.Lock()
	version := c.opened[uri] + 1
	opened := c.opened[uri] != 0
	c.opened[uri] = version
	c.mu.Unlock()

	if !opened {
		return uri, version, c.notify("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{
				"uri":        uri,
				"languageId": "pkl",
				"version":    version,
				"text":       source,
			},
		})
	}
	return uri, version, c.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": version},
		"contentChanges": []map[string]any{
			{"text": source},
		},
	})
}

func (c *client) waitForDiagnostics(ctx context.Context, uri string, version int32) ([]lspDiagnostic, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		c.mu.Lock()
		state, ok := c.diags[uri]
		c.mu.Unlock()
		if ok && state.version >= version {
			return state.diagnostics, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func resolveBinary(explicit string) (string, error) {
	candidates := []string{}
	for _, s := range []string{
		explicit,
		os.Getenv("SWITCHYARD_PKL_LSP"),
		os.Getenv("PKL_LSP_BIN"),
	} {
		if s != "" {
			candidates = append(candidates, expandHome(s))
		}
	}
	candidates = append(candidates,
		"../pkl-lsp-rust/target/release/pkl-lsp",
		"../pkl-lsp-rust/target/debug/pkl-lsp",
		"../../pkl-lsp-rust/target/release/pkl-lsp",
	)
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			abs, err := filepath.Abs(cand)
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	if path, err := exec.LookPath("pkl-lsp"); err == nil {
		return path, nil
	}
	return "", errors.New("pkl-lsp binary not found; set --pkl-lsp-path or SWITCHYARD_PKL_LSP")
}

func fileURI(path string) string {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		abs = path
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	return u.String()
}

func pathFromFileURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return raw
	}
	return filepath.FromSlash(u.Path)
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func markdownFromHover(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var h hoverResult
	if err := json.Unmarshal(raw, &h); err != nil {
		return ""
	}
	var markup struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(h.Contents, &markup); err == nil && markup.Value != "" {
		return markup.Value
	}
	var s string
	if err := json.Unmarshal(h.Contents, &s); err == nil {
		return s
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(h.Contents, &arr); err == nil {
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			if text := hoverContentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

func hoverContentText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var markup struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &markup); err == nil {
		return markup.Value
	}
	return ""
}
