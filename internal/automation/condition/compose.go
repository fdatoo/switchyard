package condition

import "context"

// AndCondition evaluates all sub-conditions with short-circuit on first false.
type AndCondition struct{ All []Evaluator }

func (c *AndCondition) Evaluate(ctx context.Context, env Env) (bool, error) {
	for _, e := range c.All {
		ok, err := e.Evaluate(ctx, env)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// OrCondition evaluates sub-conditions with short-circuit on first true.
type OrCondition struct{ Any []Evaluator }

func (c *OrCondition) Evaluate(ctx context.Context, env Env) (bool, error) {
	for _, e := range c.Any {
		ok, err := e.Evaluate(ctx, env)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// NotCondition inverts its inner condition, preserving any error.
type NotCondition struct{ Inner Evaluator }

func (c *NotCondition) Evaluate(ctx context.Context, env Env) (bool, error) {
	ok, err := c.Inner.Evaluate(ctx, env)
	if err != nil {
		return false, err
	}
	return !ok, nil
}
