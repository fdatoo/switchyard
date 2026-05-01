package cli

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	authpb "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

func newTokensCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "tokens", Short: "API token management"}
	cmd.AddCommand(newTokensCreateCmd(gf))
	cmd.AddCommand(newTokensRevokeCmd(gf))
	return cmd
}

func newTokensCreateCmd(gf *globalFlags) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Mint an API bearer token for the authenticated user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAuthServiceClient(httpClient, base)
			resp, err := svc.CreateToken(cmd.Context(),
				connect.NewRequest(&authpb.CreateTokenRequest{
					DisplayName: label,
				}))
			if err != nil {
				return renderConnectErr(err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), SecretBox.Render("TOKEN — STORE THIS NOW\n\n"+resp.Msg.GetToken()))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token id: %s\n", Identifier.Render(resp.Msg.GetTokenId()))
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "human-readable label")
	return cmd
}

func newTokensRevokeCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <token-id>",
		Short: "Revoke an API token by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewAuthServiceClient(httpClient, base)
			_, err = svc.RevokeToken(cmd.Context(),
				connect.NewRequest(&authpb.RevokeTokenRequest{TokenId: args[0]}))
			if err != nil {
				return renderConnectErr(err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), BadgeOK.Render("REVOKED"), args[0])
			return nil
		},
	}
}
