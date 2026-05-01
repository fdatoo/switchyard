package dashboard_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/dashboard"
)

type fakeBE struct {
	dashboards []*dashboard.DashboardData
}

func (f *fakeBE) List(_ context.Context) ([]dashboard.DashboardMeta, error) {
	out := make([]dashboard.DashboardMeta, 0, len(f.dashboards))
	for _, d := range f.dashboards {
		out = append(out, dashboard.DashboardMeta{Slug: d.Slug, Title: d.Title})
	}
	return out, nil
}

func (f *fakeBE) Get(_ context.Context, slug string) (*dashboard.DashboardData, error) {
	for _, d := range f.dashboards {
		if d.Slug == slug {
			return d, nil
		}
	}
	return nil, dashboard.ErrDashboardNotFound
}

func (f *fakeBE) Create(_ context.Context, slug, title string) (*dashboard.DashboardData, error) {
	d := &dashboard.DashboardData{Slug: slug, Title: title, WysiwygWritable: true}
	f.dashboards = append(f.dashboards, d)
	return d, nil
}

func (f *fakeBE) Delete(_ context.Context, slug string, _ bool) error {
	for i, d := range f.dashboards {
		if d.Slug == slug {
			f.dashboards = append(f.dashboards[:i], f.dashboards[i+1:]...)
			return nil
		}
	}
	return dashboard.ErrDashboardNotFound
}

func (f *fakeBE) SaveLayout(_ context.Context, d *dashboard.DashboardData) (*dashboard.DashboardData, string, error) {
	for i, existing := range f.dashboards {
		if existing.Slug == d.Slug {
			f.dashboards[i] = d
			return d, "corr-123", nil
		}
	}
	return nil, "", dashboard.ErrDashboardNotFound
}

func (f *fakeBE) WidgetCatalog(_ context.Context) ([]dashboard.WidgetClassInfo, error) {
	return dashboard.NewCatalog(nil).WidgetClasses(), nil
}

func TestDashboardService_GetNotFound(t *testing.T) {
	svc := dashboard.NewService(&fakeBE{}, dashboard.NewCatalog(nil))
	_, err := svc.Get(context.Background(), connect.NewRequest(&v1.GetDashboardRequest{Slug: "nope"}))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDashboardService_CreateAndGet(t *testing.T) {
	svc := dashboard.NewService(&fakeBE{}, dashboard.NewCatalog(nil))
	_, err := svc.Create(context.Background(), connect.NewRequest(&v1.CreateDashboardRequest{Slug: "test", Title: "Test"}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	resp, err := svc.Get(context.Background(), connect.NewRequest(&v1.GetDashboardRequest{Slug: "test"}))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if resp.Msg.Dashboard.Slug != "test" {
		t.Errorf("slug = %q, want test", resp.Msg.Dashboard.Slug)
	}
}

func TestDashboardService_GetWidgetCatalog(t *testing.T) {
	svc := dashboard.NewService(&fakeBE{}, dashboard.NewCatalog(nil))
	resp, err := svc.GetWidgetCatalog(context.Background(), connect.NewRequest(&v1.GetWidgetCatalogRequest{}))
	if err != nil {
		t.Fatalf("GetWidgetCatalog: %v", err)
	}
	if len(resp.Msg.Catalog.Classes) == 0 {
		t.Error("expected at least one widget class")
	}
}
