package cli

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	authpb "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

func NewAuthCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authentication, tokens, and policies",
	}
	cmd.AddCommand(newLoginCmd(gf))
	cmd.AddCommand(newLogoutCmd(gf))
	cmd.AddCommand(newWhoamiCmd(gf))
	cmd.AddCommand(newUsersCmd(gf))
	cmd.AddCommand(newBootstrapCmd(gf))
	cmd.AddCommand(newTokensCmd(gf))
	cmd.AddCommand(newExplainCmd(gf))
	cmd.AddCommand(newPoliciesCmd())
	return cmd
}

func newLoginCmd(_ *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Login (not yet implemented)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "login: browser-based login is not yet implemented")
			return nil
		},
	}
}

func newLogoutCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Invalidate the current session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewAuthServiceClient(httpClient, base)
			_, err = svc.Logout(cmd.Context(), connect.NewRequest(&authpb.LogoutRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), BadgeOK.Render("LOGGED OUT"))
			return nil
		},
	}
}

func newWhoamiCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print the current authenticated principal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewAuthServiceClient(httpClient, base)
			resp, err := svc.CurrentUser(cmd.Context(), connect.NewRequest(&authpb.CurrentUserRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			u := resp.Msg.GetUser()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  roles: %v\n",
				Identifier.Render(u.GetSlug()),
				Dim.Render(u.GetDisplayName()),
				u.GetRoles())
			return nil
		},
	}
}

func newUsersCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "User management",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewAuthServiceClient(httpClient, base)
			resp, err := svc.ListUsers(cmd.Context(), connect.NewRequest(&authpb.ListUsersRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			for _, u := range resp.Msg.GetUsers() {
				active := Dim.Render("inactive")
				if u.GetActive() {
					active = Success.Render("active")
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  roles: %v\n",
					Identifier.Render(u.GetSlug()),
					Dim.Render(u.GetDisplayName()),
					active,
					u.GetRoles())
			}
			return nil
		},
	})
	return cmd
}
