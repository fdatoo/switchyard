package widgetpack_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/fdatoo/switchyard/gen/switchyard/v1alpha1"
	"github.com/fdatoo/switchyard/internal/widgetpack"
)

func TestService_List_Empty(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	_ = store.Load(context.Background())
	svc := widgetpack.NewService(nil, store)
	resp, err := svc.List(context.Background(), connect.NewRequest(&v1.ListWidgetPacksRequest{}))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Msg.GetPacks()) != 0 {
		t.Errorf("expected 0 packs, got %d", len(resp.Msg.GetPacks()))
	}
}

func TestService_Uninstall_NotFound(t *testing.T) {
	store := widgetpack.NewStore(t.TempDir())
	_ = store.Load(context.Background())
	inst := widgetpack.NewInstaller(store, nil, nil, nil, "", nil)
	svc := widgetpack.NewService(inst, store)
	_, err := svc.Uninstall(context.Background(), connect.NewRequest(&v1.UninstallWidgetPackRequest{Name: "ghost", Version: "1.0.0"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got: %v", err)
	}
}
