package state

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fdatoo/gohome-driverkit/colorconv"
	entityv1 "github.com/fdatoo/gohome/gen/gohome/entity/v1"
)

// MergeState applies one (property, raw-value) update from a Z2M
// state-topic payload to prev and returns the new Attributes. The
// kind of prev is the contract: a Light entity's caller iterates the
// payload and accumulates updates by calling MergeState repeatedly;
// a sensor entity gets one matching property per call.
//
// Returns prev unchanged (and no error) if the property doesn't apply
// to prev's kind — callers don't need to know which properties go
// where; a state-topic payload from a multi-property device gets fanned
// out by iterating cache.entityByTopic, and each entity sees only the
// payload's keys, ignoring the ones it doesn't care about.
func MergeState(prev *entityv1.Attributes, property string, value json.RawMessage) (*entityv1.Attributes, error) {
	if prev == nil {
		return nil, errors.New("MergeState: prev is nil")
	}
	switch k := prev.Kind.(type) {
	case *entityv1.Attributes_Light:
		return mergeLight(prev, k.Light, property, value)
	case *entityv1.Attributes_NumericSensor:
		return mergeNumericSensor(prev, k.NumericSensor, property, value)
	case *entityv1.Attributes_BinarySensor:
		return mergeBinarySensor(prev, k.BinarySensor, property, value)
	default:
		return nil, fmt.Errorf("MergeState: unsupported kind %T", prev.Kind)
	}
}

func mergeLight(prev *entityv1.Attributes, light *entityv1.Light, property string, value json.RawMessage) (*entityv1.Attributes, error) {
	next := &entityv1.Light{
		On:         light.GetOn(),
		Brightness: light.GetBrightness(),
		ColorTemp:  light.GetColorTemp(),
		ColorRgb:   light.GetColorRgb(),
	}
	switch property {
	case "state":
		var s string
		if err := json.Unmarshal(value, &s); err != nil {
			return nil, fmt.Errorf("light state: %w", err)
		}
		next.On = s == "ON"
	case "brightness":
		var v uint32
		if err := json.Unmarshal(value, &v); err != nil {
			return nil, fmt.Errorf("brightness: %w", err)
		}
		next.Brightness = v
	case "color_temp":
		var v uint32
		if err := json.Unmarshal(value, &v); err != nil {
			return nil, fmt.Errorf("color_temp: %w", err)
		}
		next.ColorTemp = v
		next.ColorRgb = 0
	case "color":
		var xy struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}
		if err := json.Unmarshal(value, &xy); err != nil {
			return nil, fmt.Errorf("color: %w", err)
		}
		r, g, b := colorconv.XYToRGB(colorconv.XY{X: xy.X, Y: xy.Y})
		next.ColorRgb = colorconv.PackRGB(r, g, b)
		next.ColorTemp = 0
	default:
		return prev, nil // unknown property → no-op
	}
	return &entityv1.Attributes{
		Available: prev.GetAvailable(),
		Kind:      &entityv1.Attributes_Light{Light: next},
	}, nil
}

func mergeNumericSensor(prev *entityv1.Attributes, sensor *entityv1.NumericSensor, _ string, value json.RawMessage) (*entityv1.Attributes, error) {
	var v float64
	if err := json.Unmarshal(value, &v); err != nil {
		return nil, fmt.Errorf("numeric sensor: %w", err)
	}
	return &entityv1.Attributes{
		Available: prev.GetAvailable(),
		Kind: &entityv1.Attributes_NumericSensor{
			NumericSensor: &entityv1.NumericSensor{Unit: sensor.GetUnit(), Value: v},
		},
	}, nil
}

func mergeBinarySensor(prev *entityv1.Attributes, _ *entityv1.BinarySensor, _ string, value json.RawMessage) (*entityv1.Attributes, error) {
	// Z2M binary properties may be reported as bool OR string ("ON"/"OFF").
	// Try bool first; fall back to string.
	var b bool
	if err := json.Unmarshal(value, &b); err == nil {
		return &entityv1.Attributes{
			Available: prev.GetAvailable(),
			Kind:      &entityv1.Attributes_BinarySensor{BinarySensor: &entityv1.BinarySensor{On: b}},
		}, nil
	}
	var s string
	if err := json.Unmarshal(value, &s); err != nil {
		return nil, fmt.Errorf("binary sensor: not bool or string")
	}
	on := s == "ON" || s == "true"
	return &entityv1.Attributes{
		Available: prev.GetAvailable(),
		Kind:      &entityv1.Attributes_BinarySensor{BinarySensor: &entityv1.BinarySensor{On: on}},
	}, nil
}
