package cli

import (
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newAutomationCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "automation", Short: "Inspect and control automations"}
	c.AddCommand(
		newAutomationListCmd(gf),
		newAutomationGetCmd(gf),
		newAutomationEnableCmd(gf, true),
		newAutomationEnableCmd(gf, false),
		newAutomationTriggerCmd(gf),
		newAutomationTraceCmd(gf),
		newAutomationWatchCmd(gf),
	)
	return c
}

func newAutomationListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List automations",
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAutomationServiceClient(httpClient, base)
			resp, err := svc.List(cmd.Context(), connect.NewRequest(&v1.ListAutomationsRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			automations := resp.Msg.GetAutomations()
			if len(automations) == 0 {
				fmt.Println(Dim.Render("no automations registered"))
				return nil
			}
			fmt.Printf("%s\t%s\t%s\n", Header.Render("ID"), Header.Render("MODE"), Header.Render("ENABLED"))
			for _, a := range automations {
				en := Dim.Render("no")
				if a.GetEnabled() {
					en = Success.Render("yes")
				}
				fmt.Printf("%s\t%s\t%s\n", EntityID.Render(a.GetId()), Dim.Render(a.GetMode()), en)
			}
			return nil
		},
	}
}

func newAutomationGetCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Show an automation",
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAutomationServiceClient(httpClient, base)
			resp, err := svc.Get(cmd.Context(), connect.NewRequest(&v1.GetAutomationRequest{Id: args[0]}))
			if err != nil {
				return renderConnectErr(err)
			}
			a := resp.Msg.GetAutomation()
			fmt.Printf("%s: %s\n", Header.Render("ID"), EntityID.Render(a.GetId()))
			fmt.Printf("%s: %s\n", Header.Render("MODE"), Dim.Render(a.GetMode()))
			fmt.Printf("%s: %v\n", Header.Render("ENABLED"), a.GetEnabled())
			return nil
		},
	}
}

func newAutomationEnableCmd(gf *globalFlags, enable bool) *cobra.Command {
	use := "disable"
	if enable {
		use = "enable"
	}
	return &cobra.Command{
		Use:   use + " <id>",
		Args:  cobra.ExactArgs(1),
		Short: use + " an automation (in-memory; reverts on daemon restart)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAutomationServiceClient(httpClient, base)
			if enable {
				_, err = svc.Enable(cmd.Context(), connect.NewRequest(&v1.EnableAutomationRequest{Id: args[0]}))
			} else {
				_, err = svc.Disable(cmd.Context(), connect.NewRequest(&v1.DisableAutomationRequest{Id: args[0]}))
			}
			if err != nil {
				return renderConnectErr(err)
			}
			if enable {
				fmt.Println(Success.Render("enabled"), EntityID.Render(args[0]))
			} else {
				fmt.Println(Dim.Render("disabled"), EntityID.Render(args[0]))
			}
			fmt.Println(Dim.Render("(in-memory; reverts on daemon restart — edit Pkl for durable change)"))
			return nil
		},
	}
}

func newAutomationTriggerCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "trigger <id>",
		Args:  cobra.ExactArgs(1),
		Short: "Manually fire an automation",
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAutomationServiceClient(httpClient, base)
			_, err = svc.Trigger(cmd.Context(), connect.NewRequest(&v1.TriggerAutomationRequest{Id: args[0]}))
			if err != nil {
				return renderConnectErr(err)
			}
			fmt.Println(RunMarkerStarted.Render("▶"), "triggered", EntityID.Render(args[0]), Dim.Render("(manual)"))
			return nil
		},
	}
}

func newAutomationTraceCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "trace <automation-id> [run-id]",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Stream a run timeline from the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			automationID := args[0]
			runID := ""
			if len(args) > 1 {
				runID = args[1]
			}
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAutomationServiceClient(httpClient, base)
			stream, err := svc.Trace(cmd.Context(), connect.NewRequest(&v1.TraceAutomationRequest{
				Id:    automationID,
				RunId: runID,
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			defer func() { _ = stream.Close() }()
			var traceEvents []*v1.TraceEvent
			for stream.Receive() {
				msg := stream.Msg()
				if ev := msg.GetEvent(); ev != nil {
					traceEvents = append(traceEvents, ev)
					renderTraceEvent(ev)
				}
				// heartbeats: ignore
			}
			if err := stream.Err(); err != nil {
				return renderConnectErr(err)
			}
			fmt.Printf("\n%s %d events\n", Dim.Render("total:"), len(traceEvents))
			return nil
		},
	}
}

// renderTraceEvent prints a single proto TraceEvent to stdout.
func renderTraceEvent(e *v1.TraceEvent) {
	ts := ""
	if e.GetAt() != nil {
		ts = e.GetAt().AsTime().Format("2006-01-02T15:04:05")
	}
	meta := e.GetMetadata()
	switch e.GetKind() {
	case "automation_triggered", "script_invoked":
		line := fmt.Sprintf("%-20s %-28s", ts, e.GetKind())
		if e.GetAutomationId() != "" {
			line += "  " + EntityID.Render(e.GetAutomationId())
		}
		if meta["run_id"] != "" {
			line += "  " + ShortCorr(meta["run_id"])
		}
		if meta["trigger_kind"] != "" {
			line += "  " + Dim.Render("("+meta["trigger_kind"]+")")
		}
		fmt.Println(RunMarkerStarted.Render("▶") + " " + line)

	case "automation_finished", "script_finished":
		outcome := meta["outcome"]
		marker, style := outcomeMarker(outcome)
		label := e.GetAutomationId()
		if label == "" {
			label = meta["script_name"]
		}
		line := fmt.Sprintf("%-20s %-28s %s", ts, e.GetKind(), style.Render(outcome))
		if elapsed := meta["elapsed_ms"]; elapsed != "" {
			// try to render it; if not parseable just show raw
			line += "  " + Dim.Render(elapsed+"ms")
		}
		if label != "" {
			line += "  " + EntityID.Render(label)
		}
		if detail := e.GetDetail(); detail != "" {
			line += "  " + Error.Render(detail)
		}
		fmt.Println(style.Render(marker) + " " + line)

	case "command_issued", "command_ack", "scene_applied":
		line := fmt.Sprintf("  %-18s %-28s", ts, e.GetKind())
		if entity := meta["entity"]; entity != "" {
			line += "  " + EntityID.Render(entity)
		}
		if cmd := meta["command"]; cmd != "" {
			line += "  " + Dim.Render(cmd)
		}
		if src := meta["source"]; src != "" {
			line += "  " + Dim.Render("<source: "+src+">")
		}
		if outcome := meta["outcome"]; outcome != "" {
			line += "  " + Dim.Render(outcome)
		}
		if detail := e.GetDetail(); detail != "" {
			line += "  " + Error.Render(detail)
		}
		fmt.Println(Dim.Render("↳") + line)

	default:
		line := fmt.Sprintf("  %-18s %-28s", ts, e.GetKind())
		if entity := meta["entity"]; entity != "" {
			line += "  " + EntityID.Render(entity)
		}
		if src := meta["source"]; src != "" {
			line += "  " + Dim.Render(src)
		}
		if e.GetDetail() != "" {
			line += "  " + Dim.Render(e.GetDetail())
		}
		fmt.Println(Dim.Render(" ") + line)
	}
}

// outcomeMarker returns the display glyph and style for a run outcome string.
func outcomeMarker(outcome string) (string, lipgloss.Style) {
	switch outcome {
	case "ok":
		return "✓", RunMarkerSucceeded
	case "condition_fail", "skipped":
		return "⊘", RunMarkerSkipped
	default:
		return "✗", RunMarkerFailed
	}
}

func newAutomationWatchCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Stream automation lifecycle events",
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			evtSvc := switchyardv1alpha1connect.NewEventServiceClient(httpClient, base)
			stream, err := evtSvc.Tail(cmd.Context(), connect.NewRequest(&v1.TailEventsRequest{
				Filter: &v1.EventFilter{Kinds: []string{
					"automation_triggered", "automation_finished",
					"script_invoked", "script_finished",
				}},
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			defer func() { _ = stream.Close() }()
			fmt.Fprintln(os.Stderr, Dim.Render("streaming automation events (ctrl-c to stop)…"))
			for stream.Receive() {
				msg := stream.Msg()
				if ev := msg.GetEvent(); ev != nil {
					renderProtoEvent(cmd.OutOrStdout(), ev)
				}
			}
			if err := stream.Err(); err != nil {
				return renderConnectErr(err)
			}
			return nil
		},
	}
}
