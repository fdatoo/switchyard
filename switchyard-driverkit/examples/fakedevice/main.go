// Package main is a realistic example Carport driver: a dimmable light that
// tracks on/off and brightness state. Use it as a starting point for real drivers.
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	entityv1 "github.com/fdatoo/switchyard/gen/switchyard/entity/v1"

	"github.com/fdatoo/switchyard-driverkit/driver"
)

const entityID = "light.fake_light"

func main() {
	d := driver.New("fakedevice", "0.1.0")

	var mu sync.Mutex
	var on bool
	var brightness uint32 = 100

	if err := d.AddEntity(entityID, driver.EntitySpec{
		EntityType:   "light",
		FriendlyName: "Fake Light",
		Capabilities: []string{"turn_on", "turn_off", "set_brightness"},
	}); err != nil {
		log.Fatalf("AddEntity: %v", err)
	}

	d.OnCapability(entityID, "turn_on", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		mu.Lock()
		on = true
		b := brightness
		mu.Unlock()
		return lightAttrs(true, b), nil
	})

	d.OnCapability(entityID, "turn_off", func(_ context.Context, _ string, _ map[string]string) (*entityv1.Attributes, error) {
		mu.Lock()
		on = false
		mu.Unlock()
		return lightAttrs(false, 0), nil
	})

	d.OnCapability(entityID, "set_brightness", func(_ context.Context, _ string, args map[string]string) (*entityv1.Attributes, error) {
		mu.Lock()
		defer mu.Unlock()
		if s := args["brightness"]; s != "" {
			v, err := strconv.Atoi(s)
			if err != nil || v < 0 || v > 255 {
				return nil, fmt.Errorf("brightness must be an integer 0-255, got %q", s)
			}
			brightness = uint32(v)
		}
		return lightAttrs(on, brightness), nil
	})

	log.Fatal(d.Run(context.Background()))
}

func lightAttrs(isOn bool, b uint32) *entityv1.Attributes {
	return &entityv1.Attributes{Kind: &entityv1.Attributes_Light{
		Light: &entityv1.Light{On: isOn, Brightness: b},
	}}
}
