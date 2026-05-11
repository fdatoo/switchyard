package listener

import (
	"connectrpc.com/connect"

	"github.com/fdatoo/switchyard/gen/switchyard/activity/v1/activityv1connect"
	"github.com/fdatoo/switchyard/gen/switchyard/commandcatalog/v1/commandcatalogv1connect"
	"github.com/fdatoo/switchyard/gen/switchyard/editsession/v1/editsessionv1connect"
	"github.com/fdatoo/switchyard/gen/switchyard/replay/v1/replayv1connect"
	"github.com/fdatoo/switchyard/gen/switchyard/starlarkls/v1/starlarklsv1connect"
	"github.com/fdatoo/switchyard/gen/switchyard/v1alpha1/switchyardv1alpha1connect"
)

// Services is the set of handler implementations the listener needs.
type Services struct {
	System         switchyardv1alpha1connect.SystemServiceHandler
	Area           switchyardv1alpha1connect.AreaServiceHandler
	Zone           switchyardv1alpha1connect.ZoneServiceHandler
	Device         switchyardv1alpha1connect.DeviceServiceHandler
	Entity         switchyardv1alpha1connect.EntityServiceHandler
	Driver         switchyardv1alpha1connect.DriverServiceHandler
	Event          switchyardv1alpha1connect.EventServiceHandler
	Config         switchyardv1alpha1connect.ConfigServiceHandler
	Automation     switchyardv1alpha1connect.AutomationServiceHandler
	Script         switchyardv1alpha1connect.ScriptServiceHandler
	Scene          switchyardv1alpha1connect.SceneServiceHandler
	Dashboard      switchyardv1alpha1connect.DashboardServiceHandler
	Auth           switchyardv1alpha1connect.AuthServiceHandler
	WidgetPack     switchyardv1alpha1connect.WidgetPackServiceHandler
	CommandCatalog commandcatalogv1connect.CommandCatalogServiceHandler
	EditSession    editsessionv1connect.EditSessionServiceHandler
	StarlarkLs     starlarklsv1connect.StarlarkLsServiceHandler
	Activity       activityv1connect.ActivityServiceHandler
	Replay         replayv1connect.ReplayServiceHandler
}

// BuildRoutes returns the (path, handler) pairs to mount on the listener mux.
// NewXServiceHandler returns (string, http.Handler).
func BuildRoutes(svc Services, interceptors ...connect.Interceptor) []Route {
	opts := connect.WithInterceptors(interceptors...)
	routes := make([]Route, 0, 15)

	p, h := switchyardv1alpha1connect.NewSystemServiceHandler(svc.System, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewAreaServiceHandler(svc.Area, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewZoneServiceHandler(svc.Zone, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewDeviceServiceHandler(svc.Device, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewEntityServiceHandler(svc.Entity, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewDriverServiceHandler(svc.Driver, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewEventServiceHandler(svc.Event, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewConfigServiceHandler(svc.Config, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewAutomationServiceHandler(svc.Automation, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewScriptServiceHandler(svc.Script, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewSceneServiceHandler(svc.Scene, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewDashboardServiceHandler(svc.Dashboard, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewAuthServiceHandler(svc.Auth, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = switchyardv1alpha1connect.NewWidgetPackServiceHandler(svc.WidgetPack, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	if svc.CommandCatalog != nil {
		p, h = commandcatalogv1connect.NewCommandCatalogServiceHandler(svc.CommandCatalog, opts)
		routes = append(routes, Route{Path: p, Handler: h})
	}

	p, h = editsessionv1connect.NewEditSessionServiceHandler(svc.EditSession, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	if svc.StarlarkLs != nil {
		p, h = starlarklsv1connect.NewStarlarkLsServiceHandler(svc.StarlarkLs, opts)
		routes = append(routes, Route{Path: p, Handler: h})
	}
	if svc.Activity != nil {
		p, h = activityv1connect.NewActivityServiceHandler(svc.Activity, opts)
		routes = append(routes, Route{Path: p, Handler: h})
	}
	if svc.Replay != nil {
		p, h = replayv1connect.NewReplayServiceHandler(svc.Replay, opts)
		routes = append(routes, Route{Path: p, Handler: h})
	}

	return routes
}
