//go:build integration

package api_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/gohome/gen/gohome/v1alpha1"
	"github.com/fdatoo/gohome/internal/dashboard"
)

func TestIntegration_DashboardCRUD(t *testing.T) {
	catalog := dashboard.NewCatalog(nil)
	be := &integrationDashboardBE{catalog: catalog}
	svc := dashboard.NewService(be, catalog)

	// Create
	createResp, err := svc.Create(context.Background(), connect.NewRequest(&v1.CreateDashboardRequest{
		Slug:  "integration-test",
		Title: "Integration Test",
	}))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if createResp.Msg.Dashboard.Slug != "integration-test" {
		t.Errorf("slug = %q", createResp.Msg.Dashboard.Slug)
	}

	// Get
	getResp, err := svc.Get(context.Background(), connect.NewRequest(&v1.GetDashboardRequest{
		Slug: "integration-test",
	}))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if getResp.Msg.Dashboard.Title != "Integration Test" {
		t.Errorf("title = %q", getResp.Msg.Dashboard.Title)
	}

	// Delete
	_, err = svc.Delete(context.Background(), connect.NewRequest(&v1.DeleteDashboardRequest{
		Slug: "integration-test",
	}))
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

type integrationDashboardBE struct {
	catalog *dashboard.Catalog
	items   []*dashboard.DashboardData
}

func (b *integrationDashboardBE) List(_ context.Context) ([]dashboard.DashboardMeta, error) {
	out := make([]dashboard.DashboardMeta, len(b.items))
	for i, d := range b.items {
		out[i] = dashboard.DashboardMeta{Slug: d.Slug, Title: d.Title}
	}
	return out, nil
}

func (b *integrationDashboardBE) Get(_ context.Context, slug string) (*dashboard.DashboardData, error) {
	for _, d := range b.items {
		if d.Slug == slug {
			return d, nil
		}
	}
	return nil, dashboard.ErrDashboardNotFound
}

func (b *integrationDashboardBE) Create(_ context.Context, slug, title string) (*dashboard.DashboardData, error) {
	d := &dashboard.DashboardData{
		Slug:            slug,
		Title:           title,
		Grid:            dashboard.GridData{Columns: 12, RowHeight: 60},
		WysiwygWritable: true,
	}
	b.items = append(b.items, d)
	return d, nil
}

func (b *integrationDashboardBE) Delete(_ context.Context, slug string, _ bool) error {
	for i, d := range b.items {
		if d.Slug == slug {
			b.items = append(b.items[:i], b.items[i+1:]...)
			return nil
		}
	}
	return dashboard.ErrDashboardNotFound
}

func (b *integrationDashboardBE) SaveLayout(_ context.Context, d *dashboard.DashboardData) (*dashboard.DashboardData, string, error) {
	for i, existing := range b.items {
		if existing.Slug == d.Slug {
			b.items[i] = d
			return d, "corr-test", nil
		}
	}
	return nil, "", dashboard.ErrDashboardNotFound
}

func (b *integrationDashboardBE) WidgetCatalog(_ context.Context) ([]dashboard.WidgetClassInfo, error) {
	return b.catalog.WidgetClasses(), nil
}
