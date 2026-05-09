package daemon

import (
	"context"

	"github.com/fdatoo/switchyard/internal/dashboard"
	"github.com/fdatoo/switchyard/internal/widgetpack"
)

type dashboardBackend struct {
	packStore *widgetpack.Store
}

func newDashboardBackend(packStore *widgetpack.Store) *dashboardBackend {
	return &dashboardBackend{packStore: packStore}
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
	var packs []dashboard.InstalledPack
	if b.packStore != nil {
		view := b.packStore.ClassesView()
		for _, pv := range view {
			classes := make([]dashboard.PackClass, 0, len(pv.Classes))
			for _, c := range pv.Classes {
				classes = append(classes, dashboard.PackClass{
					Name:       c.Name,
					BundleURL:  c.BundleURL,
					BundleHash: c.BundleHash,
				})
			}
			packs = append(packs, dashboard.InstalledPack{
				Name:    pv.Name,
				Version: pv.Version,
				Classes: classes,
			})
		}
	}
	return dashboard.NewCatalog(packs).WidgetClasses(), nil
}
