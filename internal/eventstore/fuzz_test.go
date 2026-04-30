package eventstore_test

import (
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

func FuzzEventDecode(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x08, 0x96, 0x01})
	f.Fuzz(func(t *testing.T, data []byte) {
		var p eventv1.Payload
		_ = proto.Unmarshal(data, &p) // must not panic
	})
}

func FuzzFilterMatch(f *testing.F) {
	f.Add("state_changed", "light.a", "driver:x")
	f.Fuzz(func(t *testing.T, kind, entity, source string) {
		filter := eventstore.Filter{
			Kinds:    []string{kind},
			Entities: []string{entity},
			Sources:  []string{source},
			MinTs:    time.Time{},
		}
		e := eventstore.Event{Kind: kind, Entity: entity, Source: source, Timestamp: time.Now()}
		_ = filter.Matches(e)
	})
}

func FuzzFixtureParse(f *testing.F) {
	f.Add([]byte(`{"stateChanged":{"attributes":{"light":{"on":true,"brightness":100}}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var p eventv1.Payload
		_ = protojson.Unmarshal(data, &p) // must not panic
	})
}
