package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"

	"github.com/fdatoo/gohome/internal/eventstore"
)

func newEventsCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "events", Short: "Query, tail, inspect, and export events"}
	c.AddCommand(newEventsQueryCmd(gf))
	c.AddCommand(newEventsTailCmd(gf))
	c.AddCommand(newEventsInspectCmd(gf))
	c.AddCommand(newEventsExportCmd(gf))
	return c
}

func newEventsQueryCmd(gf *globalFlags) *cobra.Command {
	var kind, entity string
	var limit int
	c := &cobra.Command{
		Use:   "query",
		Short: "Historical query against the event log",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewEventServiceClient(httpClient, base)

			filter := &v1.EventFilter{}
			if kind != "" {
				filter.Kinds = []string{kind}
			}
			if entity != "" {
				filter.EntityPrefix = entity
			}

			req := &v1.QueryEventsRequest{
				Filter: filter,
				Page:   &v1.PageRequest{PageSize: uint32(limit)},
			}
			resp, err := svc.Query(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return renderConnectErr(err)
			}
			format := parseFormat(gf.Format, isTerminal(os.Stdout))
			return renderProtoEvents(os.Stdout, resp.Msg.GetEvents(), format)
		},
	}
	c.Flags().StringVar(&kind, "kind", "", "filter by kind")
	c.Flags().StringVar(&entity, "entity", "", "filter by entity ID prefix")
	c.Flags().IntVar(&limit, "limit", 100, "max events to return")
	return c
}

func newEventsTailCmd(gf *globalFlags) *cobra.Command {
	var kind, entity string
	c := &cobra.Command{
		Use:   "tail",
		Short: "Stream live events from the daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewEventServiceClient(httpClient, base)

			filter := &v1.EventFilter{}
			if kind != "" {
				filter.Kinds = []string{kind}
			}
			if entity != "" {
				filter.EntityPrefix = entity
			}

			stream, err := svc.Tail(cmd.Context(), connect.NewRequest(&v1.TailEventsRequest{
				Filter: filter,
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			defer func() { _ = stream.Close() }()
			format := parseFormat(gf.Format, isTerminal(os.Stdout))
			headerPrinted := false
			for stream.Receive() {
				msg := stream.Msg()
				ev := msg.GetEvent()
				if ev == nil {
					continue // heartbeats
				}
				if format == outFormatJSON {
					if err := renderProtoEvents(os.Stdout, []*v1.Event{ev}, format); err != nil {
						return err
					}
					continue
				}
				if !headerPrinted {
					_, _ = fmt.Fprintln(os.Stdout, Header.Render(fmt.Sprintf("%-12s  %-22s  %-30s  %-20s  %s", "TIME", "KIND", "ENTITY", "SOURCE", "CORR")))
					headerPrinted = true
				}
				renderProtoEventLine(os.Stdout, ev)
			}
			if err := stream.Err(); err != nil {
				return renderConnectErr(err)
			}
			return nil
		},
	}
	c.Flags().StringVar(&kind, "kind", "", "filter by kind")
	c.Flags().StringVar(&entity, "entity", "", "filter by entity ID prefix")
	return c
}

func newEventsInspectCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <position>",
		Short: "Show a single event in full detail (reads local DB)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			pos, err := strconv.ParseUint(args[0], 10, 64)
			dieOnError(err)
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer func() { _ = db.Close() }()

			store, err := eventstore.Open(ctx, eventstore.Config{}, db, nullLogger(), nullMetrics())
			dieOnError(err)
			events, err := store.Query(ctx, eventstore.QueryOptions{
				FromPosition: pos - 1, ToPosition: pos, Limit: 1,
			})
			dieOnError(err)
			if len(events) == 0 {
				dieOnError(fmt.Errorf("no event at position %d", pos))
			}
			dieOnError(inspectEvent(os.Stdout, events[0]))
		},
	}
}

func newEventsExportCmd(gf *globalFlags) *cobra.Command {
	var from, to uint64
	var out string
	c := &cobra.Command{
		Use:   "export",
		Short: "Export events as JSONL (reads local DB)",
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := cmd.Context()
			db, err := openReadOnlyDB(ctx, gf.DataDir)
			dieOnError(err)
			defer func() { _ = db.Close() }()

			store, err := eventstore.Open(ctx, eventstore.Config{}, db, nullLogger(), nullMetrics())
			dieOnError(err)

			w := os.Stdout
			if out != "" && out != "-" {
				f, err := os.Create(out)
				dieOnError(err)
				defer func() { _ = f.Close() }()
				w = f
			}

			cursor := from
			for {
				batch, err := store.Query(ctx, eventstore.QueryOptions{
					FromPosition: cursor, ToPosition: to, Limit: 1000,
				})
				dieOnError(err)
				if len(batch) == 0 {
					break
				}
				dieOnError(renderEvents(w, batch, outFormatJSON))
				cursor = batch[len(batch)-1].Position
			}
		},
	}
	c.Flags().Uint64Var(&from, "from", 0, "from position (exclusive)")
	c.Flags().Uint64Var(&to, "to", 0, "to position (inclusive); 0 = unbounded")
	c.Flags().StringVarP(&out, "output", "o", "-", "output file; - for stdout")
	return c
}

// renderProtoEvents renders a list of proto Event messages.
func renderProtoEvents(w io.Writer, events []*v1.Event, format outputFormat) error {
	switch format {
	case outFormatJSON:
		enc := json.NewEncoder(w)
		for _, e := range events {
			if err := enc.Encode(protoEventToMap(e)); err != nil {
				return err
			}
		}
		return nil
	default:
		return renderProtoEventsTable(w, events)
	}
}

// renderProtoEventsTable renders proto events as a table.
func renderProtoEventsTable(w io.Writer, events []*v1.Event) error {
	t := lgtable.New().
		Headers("Time", "Kind", "Entity", "Source", "Correlation").
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == 0 {
				return Header
			}
			return lipgloss.NewStyle()
		})
	for _, e := range events {
		ts := ""
		if e.GetAt() != nil {
			ts = e.GetAt().AsTime().Format("15:04:05.000")
		}
		corr := e.GetCorrelationId()
		if len(corr) > 8 {
			corr = corr[:8]
		}
		t.Row(
			ts,
			Kind.Render(e.GetKind()),
			EntityID.Render(e.GetEntity()),
			Dim.Render(e.GetSource()),
			Correlation.Render(corr),
		)
	}
	_, err := fmt.Fprintln(w, t)
	return err
}

// renderProtoEventLine writes a single event as a fixed-width line. Used by
// the streaming `events tail` so consecutive events share alignment instead
// of each rendering as a one-row table.
func renderProtoEventLine(w io.Writer, e *v1.Event) {
	ts := ""
	if e.GetAt() != nil {
		ts = e.GetAt().AsTime().Format("15:04:05.000")
	}
	corr := e.GetCorrelationId()
	if len(corr) > 8 {
		corr = corr[:8]
	}
	// Pad raw values to width, then apply styling (so ANSI escapes don't
	// throw off column alignment).
	_, _ = fmt.Fprintf(w, "%-12s  %s  %s  %s  %s\n",
		ts,
		Kind.Render(padOrTrunc(e.GetKind(), 22)),
		EntityID.Render(padOrTrunc(e.GetEntity(), 30)),
		Dim.Render(padOrTrunc(e.GetSource(), 20)),
		Correlation.Render(corr),
	)
}

func padOrTrunc(s string, width int) string {
	if len(s) > width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// protoEventToMap converts a proto Event to a JSON-serialisable map.
func protoEventToMap(e *v1.Event) map[string]any {
	ts := ""
	if e.GetAt() != nil {
		ts = e.GetAt().AsTime().Format(time.RFC3339Nano)
	}
	m := map[string]any{
		"cursor":         e.GetCursor(),
		"timestamp":      ts,
		"kind":           e.GetKind(),
		"entity":         e.GetEntity(),
		"source":         e.GetSource(),
		"correlation_id": e.GetCorrelationId(),
	}
	if e.GetCauseId() != "" {
		m["cause_id"] = e.GetCauseId()
	}
	return m
}

// renderProtoEvent renders a single proto Event to a writer (used by automation watch).
func renderProtoEvent(w io.Writer, e *v1.Event) {
	ts := ""
	if e.GetAt() != nil {
		ts = e.GetAt().AsTime().Format("15:04:05.000")
	}
	corr := e.GetCorrelationId()
	if len(corr) > 8 {
		corr = corr[:8]
	}
	parts := []string{
		Timestamp.Render(ts),
		Kind.Render(e.GetKind()),
	}
	if e.GetEntity() != "" {
		parts = append(parts, EntityID.Render(e.GetEntity()))
	}
	if e.GetSource() != "" {
		parts = append(parts, Dim.Render(e.GetSource()))
	}
	if corr != "" {
		parts = append(parts, Correlation.Render(corr))
	}
	_, _ = fmt.Fprintln(w, strings.Join(parts, "  "))
}
