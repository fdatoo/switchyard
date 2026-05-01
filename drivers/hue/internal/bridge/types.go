// Package bridge is the HTTPS + SSE client for the Philips Hue CLIP v2 API.
package bridge

// Light is a single light resource as returned by GET /clip/v2/resource/light.
// Only the fields we use are modeled; the bridge sends more.
type Light struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"` // always "light" for items in the lights collection
	Owner            ResourceRef       `json:"owner"`
	Metadata         LightMetadata     `json:"metadata"`
	On               OnState           `json:"on"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
	Color            *Color            `json:"color,omitempty"`
}

// LightMetadata carries the human-friendly bulb name set in the Hue app.
type LightMetadata struct {
	Name string `json:"name"`
}

// OnState models the bridge's nested {"on": bool} shape.
type OnState struct {
	On bool `json:"on"`
}

// Dimming carries the bulb's brightness in 0-100 float (Hue's native range).
type Dimming struct {
	Brightness float64 `json:"brightness"`
}

// ColorTemperature carries color temp in mireds. Mirek is null on bulbs that
// don't support color temperature.
type ColorTemperature struct {
	Mirek *uint32 `json:"mirek"`
}

// Dynamics carries optional transition timing for a LightUpdate. Hue v2
// caps duration at ~6,000,000 ms (100 minutes).
type Dynamics struct {
	Duration uint32 `json:"duration"`
}

// LightUpdate is the JSON body sent to PUT /clip/v2/resource/light/{id}.
// Pointer fields let us send only the keys we want to change.
type LightUpdate struct {
	On               *OnState          `json:"on,omitempty"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
	Color            *ColorUpdate      `json:"color,omitempty"`
	Dynamics         *Dynamics         `json:"dynamics,omitempty"`
}

// listLightsResponse is the envelope returned by GET /clip/v2/resource/light.
type listLightsResponse struct {
	Errors []struct {
		Description string `json:"description"`
	} `json:"errors"`
	Data []Light `json:"data"`
}

// Device is a single physical device on the bridge. Each device carries
// a list of services (light, button, zigbee_connectivity, etc.).
type Device struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Services []ResourceRef `json:"services"`
}

// ResourceRef is the {rid, rtype} pointer used pervasively in CLIP v2.
type ResourceRef struct {
	RID   string `json:"rid"`
	RType string `json:"rtype"`
}

// ZigbeeConnectivity carries the bulb's reachability status.
type ZigbeeConnectivity struct {
	ID     string      `json:"id"`
	Owner  ResourceRef `json:"owner"`
	Status string      `json:"status"` // "connected" | "connectivity_issue" | "unreachable"
}

// Event is a single resource-changed payload pulled from the SSE stream.
// Hue v2 events carry only the fields that changed.
type Event struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"` // "light" | "zigbee_connectivity"
	On               *OnState          `json:"on,omitempty"`
	Dimming          *Dimming          `json:"dimming,omitempty"`
	ColorTemperature *ColorTemperature `json:"color_temperature,omitempty"`
	Color            *Color            `json:"color,omitempty"`
	// 90-99: connectivity (when Type == "zigbee_connectivity")
	Status string       `json:"status,omitempty"`
	Owner  *ResourceRef `json:"owner,omitempty"`
}

// Color is the Hue v2 color block on a Light response. Carries the
// current xy point and the bulb's representable gamut.
type Color struct {
	XY    ColorXY `json:"xy"`
	Gamut Gamut   `json:"gamut"`
}

// ColorXY is a CIE 1931 chromaticity point. Both dimensions are 0..1.
type ColorXY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Gamut is the triangle of representable colors for one bulb model.
// Hue v2 returns the three corners explicitly.
type Gamut struct {
	Red   ColorXY `json:"red"`
	Green ColorXY `json:"green"`
	Blue  ColorXY `json:"blue"`
}

// ColorUpdate is the body shape for a PUT — only xy, no gamut.
type ColorUpdate struct {
	XY ColorXY `json:"xy"`
}
