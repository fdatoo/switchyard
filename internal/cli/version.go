package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Run: func(cmd *cobra.Command, _ []string) {
			commit := Commit
			if commit == "unknown" {
				if info, ok := debug.ReadBuildInfo(); ok {
					for _, s := range info.Settings {
						if s.Key == "vcs.revision" {
							commit = s.Value
						}
					}
				}
			}
			fmt.Printf("gohome %s (%s)\n", Version, commit)
		},
	}
}
