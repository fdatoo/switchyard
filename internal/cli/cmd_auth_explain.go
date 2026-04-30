package cli

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	authpb "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

func newExplainCmd(gf *globalFlags) *cobra.Command {
	var user, action, target, verb string
	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain why an authorization decision would be allow or deny",
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, method := splitDot(action)
			tk, ti := splitColon(target)
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			authSvc := gohomev1alpha1connect.NewAuthServiceClient(httpClient, base)
			resp, err := authSvc.ExplainAuthorization(cmd.Context(),
				connect.NewRequest(&authpb.ExplainAuthorizationRequest{
					UserSlug:      user,
					ActionService: svc,
					ActionMethod:  method,
					ActionVerb:    verb,
					TargetKind:    tk,
					TargetId:      ti,
				}))
			if err != nil {
				return renderConnectErr(err)
			}
			decision := BadgeOK.Render("ALLOWED")
			if strings.EqualFold(resp.Msg.GetDecision(), "DENIED") {
				decision = BadgeError.Render("DENIED")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Decision: %s\nReason:   %s\n", decision, resp.Msg.GetReason())
			if resp.Msg.GetRuleName() != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Rule:     %s\n", RuleName.Render(resp.Msg.GetRuleName()))
			}
			if len(resp.Msg.GetSteps()) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Trace:")
				for _, s := range resp.Msg.GetSteps() {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", s)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "user slug to check")
	cmd.Flags().StringVar(&action, "action", "", "Service.Method")
	cmd.Flags().StringVar(&target, "target", "", "kind:id (e.g. entity:lock.front_door)")
	cmd.Flags().StringVar(&verb, "verb", "call", "verb")
	return cmd
}

func splitDot(s string) (string, string) {
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+1:]
}

func splitColon(s string) (string, string) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", s
	}
	return s[:i], s[i+1:]
}
