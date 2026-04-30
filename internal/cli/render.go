package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/fdatoo/gohome/internal/eventstore"
)

type outputFormat int

const (
	outFormatAuto outputFormat = iota
	outFormatTable
	outFormatJSON
)

func parseFormat(s string, isTerminal bool) outputFormat {
	switch s {
	case "table":
		return outFormatTable
	case "json":
		return outFormatJSON
	default:
		if isTerminal {
			return outFormatTable
		}
		return outFormatJSON
	}
}

func renderEvents(w io.Writer, events []eventstore.Event, format outputFormat) error {
	switch format {
	case outFormatJSON:
		enc := json.NewEncoder(w)
		for _, e := range events {
			if err := enc.Encode(eventToMap(e)); err != nil {
				return err
			}
		}
		return nil
	default:
		return renderEventsTable(w, events)
	}
}

func renderEventsTable(w io.Writer, events []eventstore.Event) error {
	t := lgtable.New().
		Headers("Time", "Kind", "Entity", "Source", "Correlation").
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == 0 {
				return Header
			}
			return lipgloss.NewStyle()
		})
	for _, e := range events {
		corr := ""
		if e.CorrelationID != [16]byte{} {
			corr = e.CorrelationID.String()[:8]
		}
		t.Row(
			e.Timestamp.Format("15:04:05.000"),
			Kind.Render(e.Kind),
			EntityID.Render(e.Entity),
			Dim.Render(e.Source),
			Correlation.Render(corr),
		)
	}
	_, err := fmt.Fprintln(w, t)
	return err
}

func eventToMap(e eventstore.Event) map[string]any {
	m := map[string]any{
		"position":       e.Position,
		"timestamp":      e.Timestamp.Format(time.RFC3339Nano),
		"kind":           e.Kind,
		"entity":         e.Entity,
		"source":         e.Source,
		"correlation_id": e.CorrelationID.String(),
	}
	if e.CausePosition > 0 {
		m["cause_position"] = e.CausePosition
	}
	if e.Payload != nil {
		if raw, err := protojson.Marshal(e.Payload); err == nil {
			var payload any
			if err := json.Unmarshal(raw, &payload); err == nil {
				m["payload"] = payload
			} else {
				m["payload"] = string(raw)
			}
		}
	}
	return m
}

func inspectEvent(w io.Writer, e eventstore.Event) error {
	var b strings.Builder
	b.WriteString(Header.Render("Event #"+fmt.Sprint(e.Position)) + "\n")
	b.WriteString(Dim.Render("Time:        ") + e.Timestamp.Format(time.RFC3339Nano) + "\n")
	b.WriteString(Dim.Render("Kind:        ") + Kind.Render(e.Kind) + "\n")
	b.WriteString(Dim.Render("Entity:      ") + EntityID.Render(e.Entity) + "\n")
	b.WriteString(Dim.Render("Source:      ") + e.Source + "\n")
	b.WriteString(Dim.Render("Correlation: ") + Correlation.Render(e.CorrelationID.String()) + "\n")
	if e.CausePosition > 0 {
		b.WriteString(Dim.Render("Caused by:   ") + fmt.Sprint(e.CausePosition) + "\n")
	}
	if e.Payload != nil {
		raw, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(e.Payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		b.WriteString("\n" + Header.Render("Payload") + "\n")
		b.WriteString(string(raw) + "\n")
	}
	_, err := fmt.Fprint(w, b.String())
	return err
}
