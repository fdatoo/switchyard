package starlark

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	starlarktime "go.starlark.net/lib/time"
	starlarkgo "go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	eventv1 "github.com/fdatoo/gohome/gen/gohome/event/v1"
	"github.com/fdatoo/gohome/internal/eventstore"
)

// EntityState is the Starlark-visible view of an entity.
type EntityState struct {
	StateStr   string         // "on", "off", sensor value, etc.
	Attributes map[string]any // proto JSON-decoded flat attributes
}

// StateReader is satisfied by the daemon's state.Cache adapter.
type StateReader interface {
	Get(entityID string) (*EntityState, bool)
}

// DispatchResult carries the driver's response to a command.
type DispatchResult struct {
	Ok    bool
	Error string
}

// CommandDispatcher is satisfied by the daemon's carport.Host adapter.
type CommandDispatcher interface {
	Dispatch(ctx context.Context, entityID, capability string, args map[string]string) (*DispatchResult, error)
}

// EventAppender is satisfied by internal/eventstore.Store.
type EventAppender interface {
	Append(ctx context.Context, e eventstore.Event) (uint64, error)
}

// MakeStateBuiltin returns the Starlark `state(entity_id)` builtin.
// Exported so testutil and daemon wiring can construct it independently.
func MakeStateBuiltin(sr StateReader) starlarkgo.Value {
	return starlarkgo.NewBuiltin("state", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var entityID string
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "entity_id", &entityID); err != nil {
			return nil, err
		}
		es, ok := sr.Get(entityID)
		if !ok {
			return nil, fmt.Errorf("state: entity %q not found", entityID)
		}
		return entityStateToStruct(es), nil
	})
}

func entityStateToStruct(es *EntityState) *starlarkstruct.Struct {
	attrs := starlarkgo.NewDict(len(es.Attributes))
	for k, v := range es.Attributes {
		_ = attrs.SetKey(starlarkgo.String(k), anyToStarlark(v))
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlarkgo.StringDict{
		"state":      starlarkgo.String(es.StateStr),
		"attributes": attrs,
	})
}

func anyToStarlark(v any) starlarkgo.Value {
	switch t := v.(type) {
	case bool:
		return starlarkgo.Bool(t)
	case float64:
		return starlarkgo.Float(t)
	case string:
		return starlarkgo.String(t)
	case []any:
		l := make([]starlarkgo.Value, len(t))
		for i, e := range t {
			l[i] = anyToStarlark(e)
		}
		return starlarkgo.NewList(l)
	case map[string]any:
		d := starlarkgo.NewDict(len(t))
		for k, val := range t {
			_ = d.SetKey(starlarkgo.String(k), anyToStarlark(val))
		}
		return d
	default:
		return starlarkgo.None
	}
}

// MakeCallServiceBuiltin returns the `call_service(entity_id, capability, **kwargs)` builtin.
func MakeCallServiceBuiltin(ctx context.Context, d CommandDispatcher) starlarkgo.Value {
	return starlarkgo.NewBuiltin("call_service", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("call_service: expected (entity_id, capability, **kwargs)")
		}
		entityID, ok := starlarkgo.AsString(args[0])
		if !ok {
			return nil, fmt.Errorf("call_service: entity_id must be string")
		}
		capability, ok := starlarkgo.AsString(args[1])
		if !ok {
			return nil, fmt.Errorf("call_service: capability must be string")
		}
		argsMap := make(map[string]string, len(kwargs))
		for _, kv := range kwargs {
			k, _ := starlarkgo.AsString(kv[0])
			v, ok := starlarkgo.AsString(kv[1])
			if !ok {
				return nil, fmt.Errorf("call_service: kwarg %q must be string", kv[0])
			}
			argsMap[k] = v
		}
		res, err := d.Dispatch(ctx, entityID, capability, argsMap)
		if err != nil {
			return nil, err
		}
		if !res.Ok {
			return nil, fmt.Errorf("call_service %s.%s: %s", entityID, capability, res.Error)
		}
		return starlarkgo.None, nil
	})
}

func makeSleep(ctx context.Context) starlarkgo.Value {
	return starlarkgo.NewBuiltin("sleep", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var seconds starlarkgo.Float
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "seconds", &seconds); err != nil {
			return nil, err
		}
		select {
		case <-time.After(time.Duration(float64(seconds) * float64(time.Second))):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return starlarkgo.None, nil
	})
}

func makeNow() starlarkgo.Value {
	return starlarkgo.NewBuiltin("now", func(thread *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		return starlarkgo.Call(thread, starlarktime.Module.Members["now"], args, kwargs)
	})
}

func makeLog(logFn func(level, msg string)) starlarkgo.Value {
	return starlarkgo.NewBuiltin("log", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var msg starlarkgo.Value
		level := "info"
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "msg", &msg, "level?", &level); err != nil {
			return nil, err
		}
		logFn(level, msg.String())
		return starlarkgo.None, nil
	})
}

func makeNotify(ctx context.Context, store EventAppender) starlarkgo.Value {
	return starlarkgo.NewBuiltin("notify", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var target, message string
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "target", &target, "message", &message); err != nil {
			return nil, err
		}
		_, err := store.Append(ctx, eventstore.Event{
			Kind:      "notification.sent",
			Entity:    target,
			Source:    "starlark",
			Timestamp: time.Now(),
			Payload: &eventv1.Payload{
				Kind: &eventv1.Payload_System{
					System: &eventv1.SystemEvent{
						Kind: "notification.sent",
						Data: map[string]string{"target": target, "message": message},
					},
				},
			},
		})
		return starlarkgo.None, err
	})
}

func makeRandom(rng *rand.Rand) starlarkgo.Value {
	return starlarkgo.NewBuiltin("random", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs); err != nil {
			return nil, err
		}
		return starlarkgo.Float(rng.Float64()), nil
	})
}

func makeSceneGlobal(ctx context.Context, store EventAppender) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlarkgo.StringDict{
		"apply": starlarkgo.NewBuiltin("scene.apply", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
			var slug string
			if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "slug", &slug); err != nil {
				return nil, err
			}
			_, err := store.Append(ctx, eventstore.Event{
				Kind:      "scene.applied",
				Entity:    slug,
				Source:    "starlark",
				Timestamp: time.Now(),
				Payload: &eventv1.Payload{
					Kind: &eventv1.Payload_System{
						System: &eventv1.SystemEvent{
							Kind: "scene.applied",
							Data: map[string]string{"slug": slug},
						},
					},
				},
			})
			return starlarkgo.None, err
		}),
	})
}

// makeEventGlobal builds the read-write event struct for automation/script contexts.
// Trigger event fields come from extraGlobals keys "event_kind", "event_entity_id",
// "event_data" (set by C6 when invoking automations from a trigger).
func makeEventGlobal(ctx context.Context, store EventAppender, extraGlobals starlarkgo.StringDict) *starlarkstruct.Struct {
	fields := starlarkgo.StringDict{
		"fire": starlarkgo.NewBuiltin("event.fire", func(_ *starlarkgo.Thread, b *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
			var kind string
			var data starlarkgo.Value = starlarkgo.None
			if err := starlarkgo.UnpackArgs(b.Name(), args, kwargs, "kind", &kind, "data?", &data); err != nil {
				return nil, err
			}
			_, err := store.Append(ctx, eventstore.Event{
				Kind:      kind,
				Source:    "starlark",
				Timestamp: time.Now(),
				Payload: &eventv1.Payload{
					Kind: &eventv1.Payload_System{
						System: &eventv1.SystemEvent{
							Kind: kind,
							Data: starlarkDataToStringMap(data),
						},
					},
				},
			})
			return starlarkgo.None, err
		}),
		"kind":      eventField(extraGlobals, "event_kind", starlarkgo.String("")),
		"entity_id": eventField(extraGlobals, "event_entity_id", starlarkgo.String("")),
		"data":      eventField(extraGlobals, "event_data", starlarkgo.NewDict(0)),
	}
	return starlarkstruct.FromStringDict(starlarkstruct.Default, fields)
}

// starlarkDataToStringMap converts a Starlark value to a map[string]string for
// embedding in event payloads. Dict string-to-string pairs go in directly; other
// value types use their .String() representation. If v is None, returns an empty map.
func starlarkDataToStringMap(v starlarkgo.Value) map[string]string {
	result := map[string]string{}
	if v == nil || v == starlarkgo.None {
		return result
	}
	d, ok := v.(*starlarkgo.Dict)
	if !ok {
		return result
	}
	for _, item := range d.Items() {
		k, ok := starlarkgo.AsString(item[0])
		if !ok {
			k = item[0].String()
		}
		val, ok := starlarkgo.AsString(item[1])
		if !ok {
			val = item[1].String()
		}
		result[k] = val
	}
	return result
}

// makeEventGlobalReadOnly builds the read-only event struct for trigger condition contexts.
func makeEventGlobalReadOnly(extraGlobals starlarkgo.StringDict) *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(starlarkstruct.Default, starlarkgo.StringDict{
		"kind":      eventField(extraGlobals, "event_kind", starlarkgo.String("")),
		"entity_id": eventField(extraGlobals, "event_entity_id", starlarkgo.String("")),
		"data":      eventField(extraGlobals, "event_data", starlarkgo.NewDict(0)),
	})
}

func eventField(extra starlarkgo.StringDict, key string, fallback starlarkgo.Value) starlarkgo.Value {
	if v, ok := extra[key]; ok {
		return v
	}
	return fallback
}

func makeSlogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
