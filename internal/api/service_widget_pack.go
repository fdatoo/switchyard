package api

import (
	"github.com/fdatoo/switchyard/internal/auth"
)

// RegisterWidgetPackProcedures registers authz catalog entries for the four
// WidgetPackService procedures. Wired into the daemon's catalog construction
// once F-184 lands; until then this is a no-op at startup.
func RegisterWidgetPackProcedures(addProcedure func(string, auth.Action, func(any) auth.Target)) {
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/Install",
		auth.Action{Service: "widget_pack", Method: "install", Verb: "write"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/Uninstall",
		auth.Action{Service: "widget_pack", Method: "uninstall", Verb: "write"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/List",
		auth.Action{Service: "widget_pack", Method: "list", Verb: "read"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
	addProcedure(
		"/switchyard.v1alpha1.WidgetPackService/Watch",
		auth.Action{Service: "widget_pack", Method: "watch", Verb: "read"},
		func(any) auth.Target { return auth.Target{Kind: "widget_pack"} },
	)
}
