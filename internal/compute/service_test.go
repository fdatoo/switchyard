package compute_test

import (
	"context"
	"testing"

	"github.com/fdatoo/gohome/internal/compute"
)

func TestService_Eval_ReturnsErrorForUnimplemented(t *testing.T) {
	svc := compute.NewService()
	result := svc.Eval(context.Background(), compute.Request{
		DashboardSlug: "test",
		WidgetID:      "w1",
		ExprID:        "state.temperature",
	})
	if result.Error == "" {
		t.Error("expected non-empty error from stub implementation")
	}
}
