package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"connectrpc.com/connect"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/auth"
	internalmcp "github.com/fdatoo/switchyard/internal/mcp"
	"github.com/fdatoo/switchyard/internal/mcp/audit"
	"github.com/fdatoo/switchyard/internal/mcp/resources"
	"github.com/fdatoo/switchyard/internal/mcp/tools"
)

func newMCPCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server commands",
	}
	cmd.AddCommand(newMCPServeCmd(gf))
	cmd.AddCommand(newMCPToolsCmd())
	return cmd
}

func newMCPServeCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP server on stdio (for use with MCP clients like Claude Code)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			session := ulid.Make().String()

			endpoint := ResolveEndpoint(gf.Endpoint, gf.DataDir)
			client, err := internalmcp.NewClient(internalmcp.ClientOptions{
				EndpointURL: endpoint,
				SessionID:   session,
			})
			if err != nil {
				return fmt.Errorf("dial daemon: %w", err)
			}

			// Fetch config dir
			cdResp, err := client.System.GetConfigDir(ctx, connect.NewRequest(&v1.GetConfigDirRequest{}))
			if err != nil {
				fmt.Fprintf(os.Stderr, "gohome mcp: cannot reach gohomed at %s: %v\n", endpoint, err)
				return err
			}

			// Fetch MCP config
			cfgResp, err := client.System.GetMCPConfig(ctx, connect.NewRequest(&v1.GetMCPConfigRequest{}))
			if err != nil {
				return fmt.Errorf("fetch mcp config: %w", err)
			}
			caps := capsFromProto(cfgResp.Msg)

			m := nullMetrics()

			// Build resource server options + set-server callback
			resDeps := resources.Deps{
				Client:  client,
				MCPCaps: caps,
				Metrics: m,
			}
			serverOpts, setServer, shutdownResources := resources.NewServerOpts(resDeps)
			defer shutdownResources()

			// Build audit recorder
			auditRec := audit.NewRecorder(client.System)

			deps := internalmcp.Deps{
				Client:     client,
				ConfigDir:  cdResp.Msg.ConfigDir,
				MCPCaps:    caps,
				SessionID:  session,
				Version:    Version,
				Stdin:      os.Stdin,
				Stdout:     os.Stdout,
				Stderr:     os.Stderr,
				Metrics:    m,
				ServerOpts: serverOpts,
				RegisterTools: func(server *sdk.Server) {
					setServer(server)
					tools.Register(tools.Deps{
						Server:    server,
						Client:    client,
						ConfigDir: cdResp.Msg.ConfigDir,
						MCPCaps:   caps,
						SessionID: session,
						Auth:      auth.AllowAll{},
						Audit:     auditRec,
					})
					resources.Register(server, resDeps)
				},
			}

			return internalmcp.Run(ctx, deps)
		},
	}
	return cmd
}

func newMCPToolsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Print the MCP tool catalog and exit (no daemon required)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cat := tools.Catalog()
			if asJSON {
				b, err := json.MarshalIndent(cat, "", "  ")
				if err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			printToolsHuman(cmd.OutOrStdout(), cat)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit JSON instead of styled table")
	return cmd
}

func capsFromProto(resp *v1.GetMCPConfigResponse) internalmcp.MCPCaps {
	return internalmcp.MCPCaps{
		EvalResultMaxBytes:       resp.EvalResultMaxBytes,
		ReadFileMaxBytes:         resp.ReadFileMaxBytes,
		EntitySubscriptionBuffer: resp.EntitySubscriptionBuffer,
		TraceSubscriptionBuffer:  resp.TraceSubscriptionBuffer,
		TailDefaultWaitSeconds:   resp.TailDefaultWaitSeconds,
		TailMaxWaitSeconds:       resp.TailMaxWaitSeconds,
	}
}

func printToolsHuman(w io.Writer, cat []tools.ToolDescriptor) {
	for i, td := range cat {
		var badge string
		switch td.Verb {
		case "read":
			badge = BadgeRead.Render("READ")
		case "call":
			if td.Status == "unimplemented" {
				badge = BadgeWarn.Render("UNIMPLEMENTED")
			} else {
				badge = BadgeCall.Render("CALL")
			}
		case "admin":
			badge = BadgeAdmin.Render("ADMIN")
		}
		_, _ = fmt.Fprintf(w, "%s %s\n", badge, ToolNameStyle.Render(td.Name))
		_, _ = fmt.Fprintf(w, "  %s\n", SubtleText.Render(td.Summary))
		if i < len(cat)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}
	reads, calls, admins := 0, 0, 0
	for _, td := range cat {
		switch td.Verb {
		case "read":
			reads++
		case "call":
			calls++
		case "admin":
			admins++
		}
	}
	_, _ = fmt.Fprintf(w, "\n%s\n", SubtleText.Render(fmt.Sprintf("%d read  %d call  %d admin  (%d total)", reads, calls, admins, len(cat))))
}
