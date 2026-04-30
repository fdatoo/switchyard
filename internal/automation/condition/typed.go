package condition

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type StateCondition struct {
	Entity string
	Equals string
	OneOf  []string
	Not    string
}

func (c *StateCondition) Evaluate(_ context.Context, env Env) (bool, error) {
	s, ok := env.State.Get(c.Entity)
	if !ok {
		if env.Logger != nil {
			env.Logger.Warn("state cond: entity missing", "entity", c.Entity)
		}
		return false, nil
	}
	cur := s.StateStr
	switch {
	case c.Equals != "":
		return cur == c.Equals, nil
	case len(c.OneOf) > 0:
		for _, v := range c.OneOf {
			if cur == v {
				return true, nil
			}
		}
		return false, nil
	case c.Not != "":
		return cur != c.Not, nil
	default:
		return false, fmt.Errorf("state condition: no operator")
	}
}

type NumericCondition struct {
	Entity, Attribute, Op string
	Value                 float64
}

func (c *NumericCondition) Evaluate(_ context.Context, env Env) (bool, error) {
	s, ok := env.State.Get(c.Entity)
	if !ok {
		return false, nil
	}
	attr := c.Attribute
	if attr == "" {
		attr = "value"
	}
	raw, ok := s.Attributes[attr]
	if !ok {
		return false, nil
	}
	f, ok := toFloat(raw)
	if !ok {
		return false, nil
	}
	switch c.Op {
	case "lt":
		return f < c.Value, nil
	case "lte":
		return f <= c.Value, nil
	case "eq":
		return f == c.Value, nil
	case "gte":
		return f >= c.Value, nil
	case "gt":
		return f > c.Value, nil
	default:
		return false, fmt.Errorf("numeric op %q", c.Op)
	}
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		var f float64
		if _, err := fmt.Sscanf(x, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

type TimeCondition struct {
	After, Before string
	Weekdays      []string
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday,
	"sat": time.Saturday,
}

func (c *TimeCondition) Evaluate(_ context.Context, env Env) (bool, error) {
	if env.Loc == nil {
		env.Loc = time.Local
	}
	now := env.Now.In(env.Loc)
	if len(c.Weekdays) > 0 {
		hit := false
		for _, w := range c.Weekdays {
			wd, ok := weekdays[strings.ToLower(w)]
			if !ok {
				return false, fmt.Errorf("weekday %q", w)
			}
			if now.Weekday() == wd {
				hit = true
				break
			}
		}
		if !hit {
			return false, nil
		}
	}
	if c.After == "" && c.Before == "" {
		return true, nil
	}
	a, _ := parseHM(c.After)
	b, _ := parseHM(c.Before)
	cur := now.Hour()*60 + now.Minute()
	switch {
	case c.After != "" && c.Before == "":
		return cur >= a, nil
	case c.After == "" && c.Before != "":
		return cur < b, nil
	default:
		if a <= b {
			return cur >= a && cur < b, nil
		}
		return cur >= a || cur < b, nil
	}
}

func parseHM(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, err
	}
	return h*60 + m, nil
}
