package cli

import (
	"strings"
	"testing"
)

func TestNewWidgetCmd_HasSubcommands(t *testing.T) {
	cmd := newWidgetCmd(&globalFlags{})
	got := make(map[string]bool)
	for _, c := range cmd.Commands() {
		got[strings.SplitN(c.Use, " ", 2)[0]] = true
	}
	for _, want := range []string{"install", "list", "uninstall"} {
		if !got[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}
