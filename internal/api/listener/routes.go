package listener

import (
	"connectrpc.com/connect"

	"github.com/fdatoo/gohome/gen/gohome/v1alpha1/gohomev1alpha1connect"
)

// Services is the set of handler implementations the listener needs.
type Services struct {
	System     gohomev1alpha1connect.SystemServiceHandler
	Area       gohomev1alpha1connect.AreaServiceHandler
	Zone       gohomev1alpha1connect.ZoneServiceHandler
	Device     gohomev1alpha1connect.DeviceServiceHandler
	Entity     gohomev1alpha1connect.EntityServiceHandler
	Driver     gohomev1alpha1connect.DriverServiceHandler
	Event      gohomev1alpha1connect.EventServiceHandler
	Config     gohomev1alpha1connect.ConfigServiceHandler
	Automation gohomev1alpha1connect.AutomationServiceHandler
	Script     gohomev1alpha1connect.ScriptServiceHandler
	Scene      gohomev1alpha1connect.SceneServiceHandler
	Dashboard  gohomev1alpha1connect.DashboardServiceHandler
	Auth       gohomev1alpha1connect.AuthServiceHandler
}

// BuildRoutes returns the (path, handler) pairs to mount on the listener mux.
// NewXServiceHandler returns (string, http.Handler).
func BuildRoutes(svc Services, interceptors ...connect.Interceptor) []Route {
	opts := connect.WithInterceptors(interceptors...)
	routes := make([]Route, 0, 13)

	p, h := gohomev1alpha1connect.NewSystemServiceHandler(svc.System, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewAreaServiceHandler(svc.Area, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewZoneServiceHandler(svc.Zone, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewDeviceServiceHandler(svc.Device, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewEntityServiceHandler(svc.Entity, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewDriverServiceHandler(svc.Driver, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewEventServiceHandler(svc.Event, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewConfigServiceHandler(svc.Config, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewAutomationServiceHandler(svc.Automation, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewScriptServiceHandler(svc.Script, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewSceneServiceHandler(svc.Scene, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewDashboardServiceHandler(svc.Dashboard, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	p, h = gohomev1alpha1connect.NewAuthServiceHandler(svc.Auth, opts)
	routes = append(routes, Route{Path: p, Handler: h})

	return routes
}
