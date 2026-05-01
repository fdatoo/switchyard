package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
	"github.com/fdatoo/switchyard/internal/config"
)

func newConfigCmd(gf *globalFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Validate and apply Pkl configuration",
	}
	c.AddCommand(newConfigValidateCmd(gf))
	c.AddCommand(newConfigApplyCmd(gf))
	c.AddCommand(newConfigReloadCmd(gf))
	return c
}

func newConfigValidateCmd(gf *globalFlags) *cobra.Command {
	var offline bool
	var configDir string
	c := &cobra.Command{
		Use:   "validate",
		Short: "Evaluate and validate config without applying",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if offline {
				if configDir == "" {
					configDir = filepath.Join(expandHome(gf.DataDir), "config")
				}
				mainPkl := filepath.Join(configDir, "main.pkl")
				if _, err := os.Stat(mainPkl); err != nil {
					return fmt.Errorf("main.pkl not found in %s: %w", configDir, err)
				}
				_, validationErrs, err := config.ValidateOffline(cmd.Context(), configDir)
				if err != nil {
					return fmt.Errorf("config eval failed: %w", err)
				}
				if len(validationErrs) > 0 {
					for _, e := range validationErrs {
						_, _ = fmt.Fprintln(cmd.ErrOrStderr(), Error.Render("error:")+fmt.Sprintf(" %s", e.Message))
					}
					return fmt.Errorf("config invalid")
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), Success.Render("✓ Config valid")+" (offline)")
				return nil
			}

			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewConfigServiceClient(httpClient, base)
			resp, err := svc.Validate(cmd.Context(), connect.NewRequest(&v1.ValidateConfigRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			msg := resp.Msg
			if !msg.GetValid() {
				for _, e := range msg.GetErrors() {
					_, _ = fmt.Fprintln(os.Stderr, Error.Render("error:")+fmt.Sprintf(" %s", e))
				}
				return fmt.Errorf("config invalid")
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), Success.Render("✓ Config valid"))
			if d := msg.GetDiff(); d != nil {
				t := lgtable.New().StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
				t.Row("Driver instances added", fmt.Sprintf("+%d", d.GetDriverInstancesAdded()))
				t.Row("Driver instances removed", fmt.Sprintf("-%d", d.GetDriverInstancesRemoved()))
				t.Row("Driver instances changed", fmt.Sprintf("~%d", d.GetDriverInstancesChanged()))
				t.Row("Automations changed", fmt.Sprintf("~%d", d.GetAutomationsChanged()))
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), t)
				for _, line := range d.GetLines() {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), Dim.Render(line))
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&offline, "offline", false, "validate locally without connecting to the daemon")
	c.Flags().StringVar(&configDir, "config-dir", "", "config directory to validate (default: <data-dir>/config)")
	return c
}

func newConfigApplyCmd(gf *globalFlags) *cobra.Command {
	var dryRun bool
	var message string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Evaluate, validate, and apply config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewConfigServiceClient(httpClient, base)
			resp, err := svc.Apply(cmd.Context(), connect.NewRequest(&v1.ApplyConfigRequest{
				Message: message,
				DryRun:  dryRun,
			}))
			if err != nil {
				return renderConnectErr(err)
			}
			msg := resp.Msg
			label := ""
			if dryRun {
				label = Dim.Render(" (dry-run)")
			}
			fmt.Printf("Config applied%s\n", label)
			if d := msg.GetDiff(); d != nil {
				t := lgtable.New().
					Headers("Resource", "Added", "Removed", "Changed").
					StyleFunc(func(_, _ int) lipgloss.Style { return lipgloss.NewStyle() })
				t.Row("Driver instances",
					fmt.Sprintf("+%d", d.GetDriverInstancesAdded()),
					fmt.Sprintf("-%d", d.GetDriverInstancesRemoved()),
					fmt.Sprintf("~%d", d.GetDriverInstancesChanged()),
				)
				t.Row("Automations", "+0", "-0", fmt.Sprintf("~%d", d.GetAutomationsChanged()))
				fmt.Println(t)
				for _, line := range d.GetLines() {
					fmt.Println(Dim.Render(line))
				}
			}
			if id := msg.GetCorrelationId(); id != "" {
				fmt.Println(Dim.Render("correlation_id: " + id))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print diff without applying")
	cmd.Flags().StringVar(&message, "message", "", "change message recorded with apply")
	return cmd
}

func newConfigReloadCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload config from daemon's configured directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ep := ResolveEndpoint(gf.Endpoint, expandHome(gf.DataDir))
			httpClient, base, err := Dial(cmd.Context(), ep)
			if err != nil {
				return err
			}
			svc := switchyardv1alpha1connect.NewConfigServiceClient(httpClient, base)
			resp, err := svc.Reload(cmd.Context(), connect.NewRequest(&v1.ReloadConfigRequest{}))
			if err != nil {
				return renderConnectErr(err)
			}
			msg := resp.Msg
			fmt.Println(Success.Render("✓ Config reloaded"))
			if d := msg.GetDiff(); d != nil {
				lines := d.GetLines()
				if len(lines) > 0 {
					fmt.Println(strings.Join(lines, "\n"))
				}
			}
			if id := msg.GetCorrelationId(); id != "" {
				fmt.Println(Dim.Render("correlation_id: " + id))
			}
			return nil
		},
	}
}
