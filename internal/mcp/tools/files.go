package tools

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"unicode/utf8"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fdatoo/switchyard/internal/mcp/audit"
	mcpfs "github.com/fdatoo/switchyard/internal/mcp/fs"
)

// ReadConfigFileInput is the input schema for gohome__read_config_file.
type ReadConfigFileInput struct {
	Path string `json:"path"`
}

// WriteConfigFileInput is the input schema for gohome__write_config_file.
type WriteConfigFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func registerFiles(d Deps) {
	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__read_config_file",
		Description: "Read a file from the config directory.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ReadConfigFileInput) (*sdk.CallToolResult, any, error) {
		target, err := mcpfs.Resolve(d.ConfigDir, in.Path)
		if err != nil {
			if errors.Is(err, mcpfs.ErrPathEscape) {
				return nil, nil, &ToolError{Reason: "path_escape", Message: err.Error(), Cause: err}
			}
			return nil, nil, toToolError(err)
		}

		info, err := os.Stat(target)
		if err != nil {
			return nil, nil, toToolError(err)
		}
		if !info.Mode().IsRegular() {
			return nil, nil, &ToolError{Reason: "not_a_file", Message: fmt.Sprintf("%q is not a regular file", in.Path)}
		}

		if d.MCPCaps.ReadFileMaxBytes > 0 && uint32(info.Size()) > d.MCPCaps.ReadFileMaxBytes {
			return nil, nil, &ToolError{
				Reason:  "file_too_large",
				Message: fmt.Sprintf("file size %d exceeds limit %d", info.Size(), d.MCPCaps.ReadFileMaxBytes),
			}
		}

		data, err := os.ReadFile(target)
		if err != nil {
			return nil, nil, toToolError(err)
		}

		if !utf8.Valid(data) {
			return nil, nil, &ToolError{Reason: "not_utf8", Message: "file is not valid UTF-8"}
		}

		sum := sha256.Sum256(data)
		out := map[string]any{
			"path":       in.Path,
			"content":    string(data),
			"size_bytes": len(data),
			"sha256_hex": hex.EncodeToString(sum[:]),
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})

	sdk.AddTool(d.Server, &sdk.Tool{
		Name:        "gohome__write_config_file",
		Description: "Write (create or overwrite) a file in the config directory. Supports .pkl and .star files.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in WriteConfigFileInput) (*sdk.CallToolResult, any, error) {
		target, err := mcpfs.Resolve(d.ConfigDir, in.Path)
		if err != nil {
			if errors.Is(err, mcpfs.ErrPathEscape) {
				return nil, nil, &ToolError{Reason: "path_escape", Message: err.Error(), Cause: err}
			}
			return nil, nil, toToolError(err)
		}

		content := []byte(in.Content)
		if syntaxErr := mcpfs.CheckSyntax(target, content); syntaxErr != nil {
			if errors.Is(syntaxErr, mcpfs.ErrUnsupportedExtension) {
				return nil, nil, &ToolError{Reason: "unsupported_extension", Message: syntaxErr.Error(), Cause: syntaxErr}
			}
			var se *mcpfs.SyntaxError
			if errors.As(syntaxErr, &se) {
				return nil, nil, &ToolError{Reason: "syntax_error", Message: syntaxErr.Error(), Cause: syntaxErr}
			}
			return nil, nil, toToolError(syntaxErr)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, nil, toToolError(err)
		}

		if err := atomicWrite(target, content); err != nil {
			return nil, nil, toToolError(err)
		}

		sum := sha256.Sum256(content)
		sha256Hex := hex.EncodeToString(sum[:])

		if d.Audit != nil {
			if _, aerr := d.Audit.ConfigFileEdited(ctx, audit.ConfigFileEditEvent{
				SessionID: d.SessionID,
				Path:      in.Path,
				Sha256Hex: sha256Hex,
				SizeBytes: uint32(len(content)),
			}); aerr != nil {
				slog.Warn("mcp: audit ConfigFileEdited failed", "err", aerr, "path", in.Path)
			}
		}

		out := map[string]any{
			"path":       in.Path,
			"sha256_hex": sha256Hex,
			"size_bytes": len(content),
		}
		b, _ := json.Marshal(out)
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(b)}}}, nil, nil
	})
}

// atomicWrite writes data to target atomically using a temp file + rename.
func atomicWrite(target string, data []byte) error {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Errorf("rand: %w", err)
	}
	tmp := target + ".tmp." + hex.EncodeToString(suffix[:])
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open tmp: %w", err)
	}
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write: %w", werr)
	}
	if serr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("sync: %w", serr)
	}
	if cerr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close: %w", cerr)
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
