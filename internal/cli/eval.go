package cli

import (
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newEvalCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "eval <file.star>",
		Short: "Evaluate a Starlark expression against the running daemon",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewScriptServiceClient(httpClient, base)
			resp, err := svc.Eval(cmd.Context(), connect.NewRequest(&v1.EvalScriptRequest{
				Expr: string(src),
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			msg := resp.Msg
			if stdout := msg.GetStdout(); stdout != "" {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), stdout)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), Success.Render("ok"))
			if result := msg.GetResult(); result != nil {
				rv := result.GetStringValue()
				if rv != "" && rv != "None" {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), EntityID.Render(rv))
				}
			}
			return nil
		},
	}
	return c
}
