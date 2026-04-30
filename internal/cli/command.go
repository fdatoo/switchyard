package cli

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

func newCommandCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "command", Short: "Send capability invocations to driver instances"}
	c.AddCommand(newCommandSendCmd(gf))
	return c
}

func newCommandSendCmd(gf *globalFlags) *cobra.Command {
	var argPairs []string
	c := &cobra.Command{
		Use:   "send <entity> <capability>",
		Short: "Invoke <capability> on <entity> via the daemon",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]*structpb.Value{}
			for _, kv := range argPairs {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("bad --arg %q (want k=v)", kv)
				}
				params[parts[0]] = structpb.NewStringValue(parts[1])
			}

			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewEntityServiceClient(httpClient, base)
			resp, err := svc.CallCapability(cmd.Context(), connect.NewRequest(&v1.CallCapabilityRequest{
				EntityId:   args[0],
				Capability: args[1],
				Parameters: &structpb.Struct{Fields: params},
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			corrID := resp.Msg.GetCorrelationId()
			if corrID != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s.%s (%s)\n",
					Success.Render("ok:"), EntityID.Render(args[0]), args[1], corrID)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s.%s\n",
					Success.Render("ok:"), EntityID.Render(args[0]), args[1])
			}
			return nil
		},
	}
	c.Flags().StringArrayVar(&argPairs, "arg", nil, "k=v (repeatable)")
	return c
}
