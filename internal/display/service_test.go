package display

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	displayv1 "github.com/fdatoo/switchyard/gen/switchyard/display/v1"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	return NewService(dir, NewPairCodeStore())
}

func TestService_Pair_ReturnsNonEmptyCode(t *testing.T) {
	svc := newTestService(t)
	resp, err := svc.Pair(context.Background(), connect.NewRequest(&displayv1.PairRequest{}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Code, 6, "pairing code must be 6 digits")
	assert.Greater(t, resp.Msg.ExpiresAt, int64(0), "expiry must be a positive unix timestamp")
}

func TestService_RedeemPairCode_ReturnsToken(t *testing.T) {
	svc := newTestService(t)
	// First generate a code.
	pairResp, err := svc.Pair(context.Background(), connect.NewRequest(&displayv1.PairRequest{}))
	require.NoError(t, err)

	redeemResp, err := svc.RedeemPairCode(context.Background(), connect.NewRequest(&displayv1.RedeemPairCodeRequest{
		Code:       pairResp.Msg.Code,
		DeviceName: "Kitchen Wall",
	}))
	require.NoError(t, err)
	assert.NotEmpty(t, redeemResp.Msg.DisplayId)
	assert.NotEmpty(t, redeemResp.Msg.Token)
	assert.Contains(t, redeemResp.Msg.Token, "sydisp_")
}

func TestService_RedeemPairCode_SecondRedeemNotFound(t *testing.T) {
	svc := newTestService(t)
	pairResp, err := svc.Pair(context.Background(), connect.NewRequest(&displayv1.PairRequest{}))
	require.NoError(t, err)

	// First redeem succeeds.
	_, err = svc.RedeemPairCode(context.Background(), connect.NewRequest(&displayv1.RedeemPairCodeRequest{
		Code:       pairResp.Msg.Code,
		DeviceName: "TV",
	}))
	require.NoError(t, err)

	// Second redeem fails.
	_, err = svc.RedeemPairCode(context.Background(), connect.NewRequest(&displayv1.RedeemPairCodeRequest{
		Code:       pairResp.Msg.Code,
		DeviceName: "TV",
	}))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestService_Update_PersistsOverridesAndReadsBack(t *testing.T) {
	svc := newTestService(t)
	pairResp, err := svc.Pair(context.Background(), connect.NewRequest(&displayv1.PairRequest{}))
	require.NoError(t, err)

	redeemResp, err := svc.RedeemPairCode(context.Background(), connect.NewRequest(&displayv1.RedeemPairCodeRequest{
		Code:       pairResp.Msg.Code,
		DeviceName: "Bedroom",
	}))
	require.NoError(t, err)
	id := redeemResp.Msg.DisplayId

	// Update with tile overrides and alert threshold.
	_, err = svc.Update(context.Background(), connect.NewRequest(&displayv1.UpdateDisplayRequest{
		Id: id,
		Config: &displayv1.Display{
			AlertThreshold: displayv1.AlertThreshold_ALERT_THRESHOLD_HIGH,
			TileOverrides: map[string]*displayv1.FidelityOverride{
				"kitchen": {
					Width:  displayv1.TileWidth_TILE_WIDTH_WIDE,
					Scenes: 4,
					Metric: displayv1.TileMetric_TILE_METRIC_SENSOR,
				},
			},
		},
	}))
	require.NoError(t, err)

	// Read back and verify.
	getResp, err := svc.Get(context.Background(), connect.NewRequest(&displayv1.GetDisplayRequest{Id: id}))
	require.NoError(t, err)
	assert.Equal(t, displayv1.AlertThreshold_ALERT_THRESHOLD_HIGH, getResp.Msg.Display.AlertThreshold)
	require.Contains(t, getResp.Msg.Display.TileOverrides, "kitchen")
	assert.Equal(t, displayv1.TileWidth_TILE_WIDTH_WIDE, getResp.Msg.Display.TileOverrides["kitchen"].Width)
	assert.Equal(t, int32(4), getResp.Msg.Display.TileOverrides["kitchen"].Scenes)
}

func TestService_List_ReturnsPairedDisplays(t *testing.T) {
	svc := newTestService(t)

	for i := 0; i < 3; i++ {
		pairResp, err := svc.Pair(context.Background(), connect.NewRequest(&displayv1.PairRequest{}))
		require.NoError(t, err)
		_, err = svc.RedeemPairCode(context.Background(), connect.NewRequest(&displayv1.RedeemPairCodeRequest{
			Code:       pairResp.Msg.Code,
			DeviceName: "device",
		}))
		require.NoError(t, err)
	}

	listResp, err := svc.List(context.Background(), connect.NewRequest(&displayv1.ListDisplaysRequest{}))
	require.NoError(t, err)
	assert.Len(t, listResp.Msg.Displays, 3)
}

func TestService_Unpair_RemovesDisplay(t *testing.T) {
	svc := newTestService(t)
	pairResp, err := svc.Pair(context.Background(), connect.NewRequest(&displayv1.PairRequest{}))
	require.NoError(t, err)
	redeemResp, err := svc.RedeemPairCode(context.Background(), connect.NewRequest(&displayv1.RedeemPairCodeRequest{
		Code:       pairResp.Msg.Code,
		DeviceName: "TV",
	}))
	require.NoError(t, err)
	id := redeemResp.Msg.DisplayId

	_, err = svc.Unpair(context.Background(), connect.NewRequest(&displayv1.UnpairDisplayRequest{Id: id}))
	require.NoError(t, err)

	_, err = svc.Get(context.Background(), connect.NewRequest(&displayv1.GetDisplayRequest{Id: id}))
	require.Error(t, err)
	var connectErr *connect.Error
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// ensure TempDir is used — needed by go test cache invalidation
var _ = os.TempDir
