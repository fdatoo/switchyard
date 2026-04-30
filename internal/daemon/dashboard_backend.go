package daemon

import (
	"context"

	"github.com/fdatoo/gohome/internal/dashboard"
)

type dashboardBackend struct {
	catalog *dashboard.Catalog
}

func newDashboardBackend() *dashboardBackend {
	return &dashboardBackend{
		catalog: dashboard.NewCatalog(nil),
	}
}

func (b *dashboardBackend) List(_ context.Context) ([]dashboard.DashboardMeta, error) {
	return nil, nil
}

func (b *dashboardBackend) Get(_ context.Context, _ string) (*dashboard.DashboardData, error) {
	return nil, dashboard.ErrDashboardNotFound
}

func (b *dashboardBackend) Create(_ context.Context, slug, title string) (*dashboard.DashboardData, error) {
	return &dashboard.DashboardData{
		Slug:            slug,
		Title:           title,
		Grid:            dashboard.GridData{Columns: 12, RowHeight: 60},
		WysiwygWritable: true,
	}, nil
}

func (b *dashboardBackend) Delete(_ context.Context, _ string, _ bool) error {
	return dashboard.ErrDashboardNotFound
}

func (b *dashboardBackend) SaveLayout(_ context.Context, d *dashboard.DashboardData) (*dashboard.DashboardData, string, error) {
	return d, "noop", nil
}

func (b *dashboardBackend) WidgetCatalog(_ context.Context) ([]dashboard.WidgetClassInfo, error) {
	return b.catalog.WidgetClasses(), nil
}
