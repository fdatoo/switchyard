package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPoliciesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "policies", Short: "Inspect compiled policies"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show compiled policy summary (not yet implemented)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "policies list: not yet implemented")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "inspect <policy-name>",
		Short: "Pretty-print one compiled policy (not yet implemented)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "policies inspect %s: not yet implemented\n", args[0])
			return nil
		},
	})
	return cmd
}
