package cli

import (
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	authpb "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

func newBootstrapCmd(gf *globalFlags) *cobra.Command {
	var intent string
	var ttl time.Duration
	cmd := &cobra.Command{
		Use:   "bootstrap <user-slug>",
		Short: "Mint a one-time enrollment token for a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewAuthServiceClient(httpClient, base)
			resp, err := svc.MintEnrollmentToken(cmd.Context(),
				connect.NewRequest(&authpb.MintEnrollmentTokenRequest{
					UserSlug:   args[0],
					Intent:     intent,
					TtlSeconds: uint32(ttl.Seconds()),
				}))
			if err != nil {
				return renderConnectErr(err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), SecretBox.Render("ENROLLMENT TOKEN — STORE THIS NOW\n\n"+resp.Msg.GetToken()))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Expires: %s\n", time.Unix(resp.Msg.GetExpiresAt(), 0).Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&intent, "intent", "register_passkey", "register_passkey | set_password")
	cmd.Flags().DurationVar(&ttl, "ttl", time.Hour, "token TTL")
	return cmd
}
