package cli

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

func newDriverCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{Use: "driver", Short: "Inspect and control driver instances"}
	c.AddCommand(newDriverListCmd(gf))
	c.AddCommand(newDriverStatusCmd(gf))
	c.AddCommand(newDriverRestartCmd(gf))
	return c
}

func newDriverListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List driver instances",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewDriverServiceClient(httpClient, base)
			resp, err := svc.ListInstances(cmd.Context(), connect.NewRequest(&v1.ListInstancesRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			t := lgtable.New().
				Headers("Instance", "Driver", "Status", "Entities", "Last Handshake").
				StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
			for _, di := range resp.Msg.GetInstances() {
				hsStr := "-"
				if di.GetLastHandshake() != nil {
					hsStr = di.GetLastHandshake().AsTime().Format("2006-01-02 15:04:05")
				}
				t.Row(
					EntityID.Render(di.GetId()),
					di.GetDriverName(),
					di.GetStatus(),
					fmt.Sprintf("%d", di.GetEntityCount()),
					hsStr,
				)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), t)
			return nil
		},
	}
}

func newDriverStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status <instance>",
		Short: "Show health status for one driver instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewDriverServiceClient(httpClient, base)
			resp, err := svc.InstanceHealth(cmd.Context(), connect.NewRequest(&v1.InstanceHealthRequest{
				InstanceId: args[0],
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			msg := resp.Msg
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "instance: %s\n", EntityID.Render(args[0]))
			okStr := Success.Render("ok")
			if !msg.GetOk() {
				okStr = Error.Render("unhealthy")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "health:   %s\n", okStr)
			if detail := msg.GetDetail(); detail != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "detail:   %s\n", Error.Render(detail))
			}
			return nil
		},
	}
}

func newDriverRestartCmd(gf *globalFlags) *cobra.Command {
	var reason string
	c := &cobra.Command{
		Use:   "restart <instance>",
		Short: "Force-restart a driver instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := gohomev1alpha1connect.NewDriverServiceClient(httpClient, base)
			_, err = svc.RestartInstance(cmd.Context(), connect.NewRequest(&v1.RestartInstanceRequest{
				InstanceId: args[0],
				Reason:     reason,
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			fmt.Printf("%s restart scheduled for %s\n", Success.Render("ok:"), EntityID.Render(args[0]))
			return nil
		},
	}
	c.Flags().StringVar(&reason, "reason", "manual", "reason for restart")
	return c
}
