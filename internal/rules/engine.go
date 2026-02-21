package rules

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/internal/config"
	"kube-watcher/internal/events"
	"kube-watcher/internal/tracker"
)

type ActionInvocation struct {
	RuleName string
	ActionID string
	Action   config.Action
	Context  map[string]interface{}
}

type Engine struct {
	cfg      *config.WatchConfig
	store    tracker.Store
	logger   *slog.Logger
	celEnv   *cel.Env
	timers   map[string]time.Time
	timersMu sync.Mutex
}

func NewEngine(logger *slog.Logger, cfg *config.WatchConfig, store tracker.Store, celEnv *cel.Env) *Engine {
	return &Engine{
		cfg:    cfg,
		store:  store,
		logger: logger,
		celEnv: celEnv,
		timers: make(map[string]time.Time),
	}
}

func (e *Engine) Evaluate(ctx context.Context, evt events.ResourceEvent) ([]ActionInvocation, error) {
	var invocations []ActionInvocation
	prev, _ := e.store.Get(evt.Key())
	currentVals := make(map[string]interface{})

	for _, rule := range e.cfg.Rules {
		if !strings.EqualFold(rule.Kind, evt.Kind) {
			continue
		}
		if rule.Namespace != "" && rule.Namespace != evt.Namespace {
			continue
		}
		ok, err := e.evaluateRule(rule, evt, prev, currentVals)
		if err != nil {
			e.logger.Warn("rule evaluation failed", "rule", rule.Name, "error", err)
			continue
		}
		if !ok {
			continue
		}
		for _, actionID := range rule.Actions {
			action, ok := e.cfg.Actions[actionID]
			if !ok {
				continue
			}
			invocations = append(invocations, ActionInvocation{
				RuleName: rule.Name,
				ActionID: actionID,
				Action:   action,
				Context: map[string]interface{}{
					"resource": evt.Object,
					"rule":     rule,
					"changes":  currentVals,
				},
			})
		}
	}

	e.store.Set(evt.Key(), tracker.Snapshot{
		ResourceVersion: evt.ResourceVersionValue(),
		Values:          currentVals,
	})
	return invocations, nil
}

func (e *Engine) evaluateRule(rule config.Rule, evt events.ResourceEvent, prev tracker.Snapshot, collected map[string]interface{}) (bool, error) {
	if len(rule.Conditions) == 0 {
		return false, nil
	}

	logicAll := !strings.EqualFold(rule.Logic, "any")
	satisfiedAny := false

	for _, cond := range rule.Conditions {
		ok, val, err := e.evaluateCondition(cond, evt, prev)
		if err != nil {
			return false, err
		}
		if cond.Field != "" {
			collected[cond.Field] = val
		}
		if logicAll && !ok {
			e.resetTimer(rule, evt)
			return false, nil
		}
		if !logicAll && ok {
			satisfiedAny = true
		}
	}

	conditionsMet := logicAll || satisfiedAny
	if !conditionsMet {
		e.resetTimer(rule, evt)
		return false, nil
	}
	if rule.For <= 0 {
		return true, nil
	}
	key := fmt.Sprintf("%s|%s", evt.Key(), rule.Name)
	e.timersMu.Lock()
	start, found := e.timers[key]
	if !found {
		e.timers[key] = time.Now()
		e.timersMu.Unlock()
		return false, nil
	}
	if time.Since(start) >= rule.For {
		delete(e.timers, key)
		e.timersMu.Unlock()
		return true, nil
	}
	e.timersMu.Unlock()
	return false, nil
}

func (e *Engine) resetTimer(rule config.Rule, evt events.ResourceEvent) {
	if rule.For <= 0 {
		return
	}
	key := fmt.Sprintf("%s|%s", evt.Key(), rule.Name)
	e.timersMu.Lock()
	delete(e.timers, key)
	e.timersMu.Unlock()
}

func (e *Engine) evaluateCondition(cond config.Condition, evt events.ResourceEvent, prev tracker.Snapshot) (bool, interface{}, error) {
	if cond.Expression != "" && e.celEnv != nil {
		// Future: evaluate CEL expressions
	}
	value, err := extractValue(evt.Object, cond.Field)
	if err != nil {
		return false, nil, err
	}
	switch strings.ToLower(cond.Operator) {
	case "eq":
		return compareScalar(value, cond.Value) == 0, value, nil
	case "ne":
		return compareScalar(value, cond.Value) != 0, value, nil
	case "gt":
		return compareScalar(value, cond.Value) > 0, value, nil
	case "lt":
		return compareScalar(value, cond.Value) < 0, value, nil
	case "changed":
		prevVal, _ := prev.Values[cond.Field]
		return !valuesEqual(prevVal, value), value, nil
	default:
		return false, value, fmt.Errorf("unsupported operator %s", cond.Operator)
	}
}

func compareScalar(a interface{}, b interface{}) int {
	aFloat, aOk := toFloat(a)
	bFloat, bOk := toFloat(b)
	if aOk && bOk {
		switch {
		case aFloat < bFloat:
			return -1
		case aFloat > bFloat:
			return 1
		default:
			return 0
		}
	}
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	switch {
	case aStr < bStr:
		return -1
	case aStr > bStr:
		return 1
	default:
		return 0
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		q, err := resource.ParseQuantity(t)
		if err == nil {
			return q.AsApproximateFloat64(), true
		}
		return 0, false
	default:
		return 0, false
	}
}

func valuesEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func extractValue(obj map[string]interface{}, path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("empty field path")
	}
	segments := strings.Split(path, ".")
	var current interface{} = obj
	for _, seg := range segments {
		seg = strings.TrimSuffix(seg, "[*]")
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("segment %s not found", seg)
		}
		current, ok = m[seg]
		if !ok {
			return nil, fmt.Errorf("field %s missing", seg)
		}
	}
	return current, nil
}
