package cli

import (
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newScriptCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "script", Short: "Inspect and run scripts"}
	c.AddCommand(newScriptListCmd(gf), newScriptRunCmd(gf))
	return c
}

func newScriptListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List scripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewScriptServiceClient(httpClient, base)
			resp, err := svc.List(cmd.Context(), connect.NewRequest(&v1.ListScriptsRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			scripts := resp.Msg.GetScripts()
			if len(scripts) == 0 {
				fmt.Println(Dim.Render("no scripts registered"))
				return nil
			}
			for _, s := range scripts {
				fmt.Println(EntityID.Render(s.GetName()))
			}
			return nil
		},
	}
}

func newScriptRunCmd(gf *globalFlags) *cobra.Command {
	var argFlags []string
	c := &cobra.Command{
		Use:   "run <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Run a script",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed := map[string]any{}
			for _, f := range argFlags {
				i := strings.IndexByte(f, '=')
				if i < 0 {
					fmt.Fprintln(os.Stderr, Error.Render("--arg expects key=value"))
					os.Exit(2)
				}
				parsed[f[:i]] = f[i+1:]
			}
			argsStruct, err := structpb.NewStruct(parsed)
			if err != nil {
				return fmt.Errorf("build args struct: %w", err)
			}
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewScriptServiceClient(httpClient, base)
			resp, err := svc.Run(cmd.Context(), connect.NewRequest(&v1.RunScriptRequest{
				Name: args[0],
				Args: argsStruct,
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			r := resp.Msg
			fmt.Println(Success.Render("ok"))
			if r.GetResult() != nil {
				rv := r.GetResult().GetStringValue()
				if rv != "" && rv != "None" {
					fmt.Println(EntityID.Render(rv))
				}
			}
			fmt.Println(Dim.Render("run_id: " + r.GetRunId()))
			return nil
		},
	}
	c.Flags().StringSliceVar(&argFlags, "arg", nil, "Script argument key=value (repeatable)")
	return c
}
