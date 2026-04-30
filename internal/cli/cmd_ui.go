package cli

import (
	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "UI development tools",
	}
	cmd.AddCommand(newUIDevCmd())
	return cmd
}

func newUIDevCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dev",
		Short: "Start the Vite dev server proxied to gohomed",
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
}
