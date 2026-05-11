package daemon

import (
	"context"

	"github.com/fdatoo/switchyard/internal/dashboard"
	"github.com/fdatoo/switchyard/internal/dashboard/pklfs"
	"github.com/fdatoo/switchyard/internal/widgetpack"
)

type dashboardBackend struct {
	*pklfs.Backend
	packStore *widgetpack.Store
}

func newDashboardBackend(configDir, driversDir string, packStore *widgetpack.Store) *dashboardBackend {
	return &dashboardBackend{
		Backend:   pklfs.New(configDir, driversDir),
		packStore: packStore,
	}
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
