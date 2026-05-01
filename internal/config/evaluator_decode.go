package config

import (
	"encoding/json"
	"fmt"
	"time"

	configpb "github.com/fdatoo/switchyard/gen/switchyard/config/v1"
)

func parseAutomationMode(s string) configpb.AutomationConfig_Mode {
	switch s {
	case "single":
		return configpb.AutomationConfig_MODE_SINGLE
	case "queued":
		return configpb.AutomationConfig_MODE_QUEUED
	case "restart":
		return configpb.AutomationConfig_MODE_RESTART
	case "parallel":
		return configpb.AutomationConfig_MODE_PARALLEL
	default:
		return configpb.AutomationConfig_MODE_SINGLE
	}
}

func parseScriptParamType(s string) configpb.ScriptParam_Type {
	switch s {
	case "string":
		return configpb.ScriptParam_TYPE_STRING
	case "int":
		return configpb.ScriptParam_TYPE_INT
	case "float":
		return configpb.ScriptParam_TYPE_FLOAT
	case "bool":
		return configpb.ScriptParam_TYPE_BOOL
	case "entity_id":
		return configpb.ScriptParam_TYPE_ENTITY_ID
	default:
		return configpb.ScriptParam_TYPE_UNSPECIFIED
	}
}

// decodeTrigger dispatches on the Pkl `_type` field (fully-qualified class path).
func decodeTrigger(raw json.RawMessage) (*configpb.TriggerConfig, error) {
	var head typedNode
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("trigger header: %w", err)
	}
	switch head.Type {
	case "gohome.automations#StateChangeTrigger":
		var s struct {
			Entities []string `json:"entities"`
			From     string   `json:"from"`
			To       string   `json:"to"`
			ForDur   string   `json:"forDur"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		dur, err := parsePklDuration(s.ForDur)
		if err != nil {
			return nil, fmt.Errorf("forDur: %w", err)
		}
		return &configpb.TriggerConfig{
			Kind: &configpb.TriggerConfig_StateChange{StateChange: &configpb.StateChangeTrigger{
				Entities: s.Entities,
				From:     s.From,
				To:       s.To,
				ForDurNs: int64(dur),
			}},
		}, nil
	case "gohome.automations#EventTrigger":
		var s struct {
			Kind string            `json:"kind"`
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.TriggerConfig{
			Kind: &configpb.TriggerConfig_Event{Event: &configpb.EventTrigger{Kind: s.Kind, Data: s.Data}},
		}, nil
	case "gohome.automations#TimeTrigger":
		var s struct {
			At    string `json:"at"`
			Cron  string `json:"cron"`
			Every string `json:"every"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		var everyNs int64
		if s.Every != "" {
			d, err := parsePklDuration(s.Every)
			if err != nil {
				return nil, fmt.Errorf("every: %w", err)
			}
			everyNs = int64(d)
		}
		return &configpb.TriggerConfig{
			Kind: &configpb.TriggerConfig_Time{Time: &configpb.TimeTrigger{At: s.At, Cron: s.Cron, EveryNs: everyNs}},
		}, nil
	case "gohome.automations#WebhookTrigger":
		var s struct {
			Path    string   `json:"path"`
			Methods []string `json:"methods"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.TriggerConfig{
			Kind: &configpb.TriggerConfig_Webhook{Webhook: &configpb.WebhookTrigger{Path: s.Path, Methods: s.Methods}},
		}, nil
	default:
		return nil, fmt.Errorf("unknown trigger type %q", head.Type)
	}
}

func decodeCondition(raw json.RawMessage) (*configpb.ConditionConfig, error) {
	var head typedNode
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	switch head.Type {
	case "gohome.automations#StateCondition":
		var s struct {
			Entity string   `json:"entity"`
			Equals string   `json:"equals"`
			OneOf  []string `json:"oneOf"`
			Not    string   `json:"not"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_State{State: &configpb.StateCondition{
			Entity: s.Entity, Equals: s.Equals, OneOf: s.OneOf, Not: s.Not,
		}}}, nil
	case "gohome.automations#NumericCondition":
		var s struct {
			Entity    string  `json:"entity"`
			Attribute string  `json:"attribute"`
			Op        string  `json:"op"`
			Value     float64 `json:"value"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Numeric{Numeric: &configpb.NumericCondition{
			Entity: s.Entity, Attribute: s.Attribute, Op: s.Op, Value: s.Value,
		}}}, nil
	case "gohome.automations#TimeCondition":
		var s struct {
			After    string   `json:"after"`
			Before   string   `json:"before"`
			Weekdays []string `json:"weekdays"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Time{Time: &configpb.TimeCondition{
			After: s.After, Before: s.Before, Weekdays: s.Weekdays,
		}}}, nil
	case "gohome.automations#StarlarkCondition":
		var s struct {
			Expr string `json:"expr"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Starlark{Starlark: &configpb.StarlarkCondition{Expr: s.Expr}}}, nil
	case "gohome.automations#AndCondition":
		var s struct {
			All []json.RawMessage `json:"all"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		andc := &configpb.AndCondition{}
		for _, r := range s.All {
			c, err := decodeCondition(r)
			if err != nil {
				return nil, err
			}
			andc.All = append(andc.All, c)
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_And{And: andc}}, nil
	case "gohome.automations#OrCondition":
		var s struct {
			Any []json.RawMessage `json:"any"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		orc := &configpb.OrCondition{}
		for _, r := range s.Any {
			c, err := decodeCondition(r)
			if err != nil {
				return nil, err
			}
			orc.Any = append(orc.Any, c)
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Or{Or: orc}}, nil
	case "gohome.automations#NotCondition":
		var s struct {
			Not json.RawMessage `json:"not"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		inner, err := decodeCondition(s.Not)
		if err != nil {
			return nil, err
		}
		return &configpb.ConditionConfig{Kind: &configpb.ConditionConfig_Not{Not: &configpb.NotCondition{Not: inner}}}, nil
	default:
		return nil, fmt.Errorf("unknown condition type %q", head.Type)
	}
}

func decodeAction(raw json.RawMessage) (*configpb.ActionConfig, error) {
	var head struct {
		Type            string `json:"_type"`
		ContinueOnError bool   `json:"continueOnError"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	switch head.Type {
	case "gohome.automations#CallServiceAction":
		var s struct {
			Entity     string            `json:"entity"`
			Capability string            `json:"capability"`
			Args       map[string]string `json:"args"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind: &configpb.ActionConfig_CallService{CallService: &configpb.CallServiceAction{
				Entity: s.Entity, Capability: s.Capability, Args: s.Args,
			}},
		}, nil
	case "gohome.automations#SceneAction":
		var s struct {
			Slug string `json:"slug"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind:            &configpb.ActionConfig_Scene{Scene: &configpb.SceneAction{Slug: s.Slug}},
		}, nil
	case "gohome.automations#ScriptAction":
		var s struct {
			Name string            `json:"name"`
			Args map[string]string `json:"args"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind:            &configpb.ActionConfig_Script{Script: &configpb.ScriptAction{Name: s.Name, Args: s.Args}},
		}, nil
	case "gohome.automations#StarlarkAction":
		var s struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind:            &configpb.ActionConfig_Starlark{Starlark: &configpb.StarlarkAction{Body: s.Body}},
		}, nil
	case "gohome.automations#WaitAction":
		var s struct {
			Duration string `json:"duration"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		d, err := parsePklDuration(s.Duration)
		if err != nil {
			return nil, fmt.Errorf("duration: %w", err)
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind:            &configpb.ActionConfig_Wait{Wait: &configpb.WaitAction{DurationNs: int64(d)}},
		}, nil
	case "gohome.automations#SequenceBlock":
		var s struct {
			Actions []json.RawMessage `json:"actions"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		blk := &configpb.SequenceBlock{}
		for _, r := range s.Actions {
			inner, err := decodeAction(r)
			if err != nil {
				return nil, err
			}
			blk.Actions = append(blk.Actions, inner)
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind:            &configpb.ActionConfig_Sequence{Sequence: blk},
		}, nil
	case "gohome.automations#ParallelBlock":
		var s struct {
			Actions []json.RawMessage `json:"actions"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		blk := &configpb.ParallelBlock{}
		for _, r := range s.Actions {
			inner, err := decodeAction(r)
			if err != nil {
				return nil, err
			}
			blk.Actions = append(blk.Actions, inner)
		}
		return &configpb.ActionConfig{
			ContinueOnError: head.ContinueOnError,
			Kind:            &configpb.ActionConfig_Parallel{Parallel: blk},
		}, nil
	default:
		return nil, fmt.Errorf("unknown action type %q", head.Type)
	}
}

// parsePklDuration accepts the Pkl-rendered duration shape.
// Pkl renders Duration as the string "<number>.<unit>" (e.g. "5.s", "1.5.m").
// Empty string yields 0.
func parsePklDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	// Pkl's JSON rendering of Duration is an object {"value":N,"unit":"s"}
	// in some versions; try that first, then fall back to string.
	var obj struct {
		Value float64 `json:"value"`
		Unit  string  `json:"unit"`
	}
	if err := json.Unmarshal([]byte(s), &obj); err == nil && obj.Unit != "" {
		unit, ok := durationUnits[obj.Unit]
		if !ok {
			return 0, fmt.Errorf("unknown duration unit %q", obj.Unit)
		}
		return time.Duration(obj.Value * float64(unit)), nil
	}
	return parseDurationToken(s)
}

var durationUnits = map[string]time.Duration{
	"ns":  time.Nanosecond,
	"us":  time.Microsecond,
	"ms":  time.Millisecond,
	"s":   time.Second,
	"m":   time.Minute,
	"min": time.Minute,
	"h":   time.Hour,
	"d":   24 * time.Hour,
}

func parseDurationToken(s string) (time.Duration, error) {
	// Accept "5.s", "1.5.m", "30.ms", "2.h"
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			numStr := s[:i]
			unitStr := s[i+1:]
			unit, ok := durationUnits[unitStr]
			if !ok {
				return 0, fmt.Errorf("unknown duration unit %q in %q", unitStr, s)
			}
			var f float64
			if _, err := fmt.Sscanf(numStr, "%f", &f); err != nil {
				return 0, fmt.Errorf("parse duration magnitude %q: %w", numStr, err)
			}
			return time.Duration(f * float64(unit)), nil
		}
	}
	return 0, fmt.Errorf("cannot parse duration %q", s)
}
