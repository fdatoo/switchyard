package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type globalFlags struct {
	DataDir  string
	Format   string
	NoColor  bool
	LogLevel string
	Verbose  bool
	Endpoint string
}

// NewRoot constructs the full command tree.
func NewRoot() *cobra.Command {
	gf := &globalFlags{}
	root := &cobra.Command{
		Use:   "switchyard",
		Short: "switchyard CLI — read-only inspection and operator ops",
		Long:  "Event-sourced home automation. Query the event log, inspect state, manage snapshots.",
	}
	root.PersistentFlags().StringVar(&gf.DataDir, "data-dir", defaultDataDir(), "data directory")
	root.PersistentFlags().StringVar(&gf.Format, "format", "auto", "auto|table|json|yaml")
	root.PersistentFlags().BoolVar(&gf.NoColor, "no-color", false, "disable ANSI color")
	root.PersistentFlags().StringVar(&gf.LogLevel, "log-level", "warn", "error|warn|info|debug")
	root.PersistentFlags().BoolVarP(&gf.Verbose, "verbose", "v", false, "--log-level=debug shortcut")
	root.PersistentFlags().StringVar(&gf.Endpoint, "endpoint", "", "API endpoint (unix:///path or tcp://host:port)")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newSystemCmd(gf))
	root.AddCommand(newEventsCmd(gf))
	root.AddCommand(newStateCmd(gf))
	root.AddCommand(newRegistryCmd(gf))
	root.AddCommand(newSnapshotCmd(gf))
	root.AddCommand(newDriverCmd(gf))
	root.AddCommand(newCommandCmd(gf))
	root.AddCommand(newConfigCmd(gf))
	root.AddCommand(newEvalCmd(gf))
	root.AddCommand(newTestCmd(gf))
	root.AddCommand(newAutomationCmd(gf))
	root.AddCommand(newScriptCmd(gf))
	root.AddCommand(newMCPCmd(gf))
	root.AddCommand(NewAuthCmd(gf))
	root.AddCommand(newWidgetCmd())
	root.AddCommand(newUICmd())
	return root
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".switchyard"
	}
	return filepath.Join(home, ".local", "share", "switchyard")
}

func dieOnError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, Error.Render("error:")+" "+err.Error())
	os.Exit(1)
}
