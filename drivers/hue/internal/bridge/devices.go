package bridge

import (
	"context"
	"fmt"
)

// listDevicesResponse and listZCResponse are the envelopes for the two
// CLIP v2 endpoints we walk to compute per-light reachability.
type listDevicesResponse struct {
	Errors []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Data []Device `json:"data"`
}

type listZCResponse struct {
	Errors []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Data []ZigbeeConnectivity `json:"data"`
}

// ListDevices returns a map of light-resource-id → reachability status
// (one of "connected" / "connectivity_issue" / "unreachable"). Walks
// /clip/v2/resource/device and /clip/v2/resource/zigbee_connectivity,
// joining via the device's services list. Lights without a corresponding
// device or zigbee_connectivity entry are absent from the result.
func (c *Client) ListDevices(ctx context.Context) (map[string]string, error) {
	var devicesEnv listDevicesResponse
	if err := c.getJSON(ctx, "/clip/v2/resource/device", &devicesEnv); err != nil {
		return nil, fmt.Errorf("hue: list devices: %w", err)
	}
	if len(devicesEnv.Errors) > 0 {
		return nil, fmt.Errorf("hue: list devices: %s", devicesEnv.Errors[0].Description)
	}

	var zcEnv listZCResponse
	if err := c.getJSON(ctx, "/clip/v2/resource/zigbee_connectivity", &zcEnv); err != nil {
		return nil, fmt.Errorf("hue: list zigbee_connectivity: %w", err)
	}
	if len(zcEnv.Errors) > 0 {
		return nil, fmt.Errorf("hue: list zigbee_connectivity: %s", zcEnv.Errors[0].Description)
	}

	// device_id → status.
	deviceStatus := make(map[string]string, len(zcEnv.Data))
	for _, zc := range zcEnv.Data {
		deviceStatus[zc.Owner.RID] = zc.Status
	}

	// light_id → status, derived by walking each device's services.
	out := make(map[string]string, len(devicesEnv.Data))
	for _, d := range devicesEnv.Data {
		var lightID string
		for _, s := range d.Services {
			if s.RType == "light" {
				lightID = s.RID
				break
			}
		}
		if lightID == "" {
			continue
		}
		out[lightID] = deviceStatus[d.ID]
	}
	return out, nil
}
