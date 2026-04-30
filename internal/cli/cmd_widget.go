package cli

import (
	"github.com/spf13/cobra"
)

func newWidgetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "widget",
		Short: "Manage widget packs",
	}
	cmd.AddCommand(newWidgetInstallCmd())
	cmd.AddCommand(newWidgetListCmd())
	cmd.AddCommand(newWidgetUninstallCmd())
	return cmd
}

func newWidgetInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <ref>",
		Short: "Install a widget pack from an OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newWidgetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed widget packs",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}

func newWidgetUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Uninstall a widget pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}
