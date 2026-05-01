package cli

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newSystemCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "system", Short: "Daemon system commands"}
	c.AddCommand(newSystemVersionCmd(gf))
	c.AddCommand(newSystemHealthCmd(gf))
	return c
}

func newSystemVersionCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show daemon version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewSystemServiceClient(httpClient, base)
			resp, err := svc.Version(cmd.Context(), connect.NewRequest(&v1.VersionRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), Success.Render(resp.Msg.BinaryVersion))
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), Dim.Render(fmt.Sprintf("commit %s · built %s · schema %s",
				resp.Msg.GitCommit, resp.Msg.BuildDate, resp.Msg.SchemaVersion)))
			return nil
		},
	}
}

func newSystemHealthCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Show daemon health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), defaultRPCTimeout)
			defer cancel()
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(ctx, ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewSystemServiceClient(httpClient, base)
			resp, err := svc.Health(ctx, connect.NewRequest(&v1.HealthRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			out := cmd.OutOrStdout()
			if resp.Msg.Ok {
				_, _ = fmt.Fprintln(out, Success.Render("OK"))
			} else {
				_, _ = fmt.Fprintln(out, Error.Render("DEGRADED"))
			}
			_, _ = fmt.Fprintln(out, Dim.Render(resp.Msg.Summary))
			for _, sub := range resp.Msg.Subsystems {
				marker := Success.Render("✓")
				if !sub.Ok {
					marker = Error.Render("✗")
				}
				_, _ = fmt.Fprintf(out, "  %s %s — %s\n", marker, sub.Name, sub.Detail)
			}
			return nil
		},
	}
}
