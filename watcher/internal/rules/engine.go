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

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/tracker"
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
	prev, _ := e.store.Get(evt.Key())
	currentVals := make(map[string]interface{})
	var invs []ActionInvocation

	for _, rule := range e.cfg.Rules {
		if !strings.EqualFold(rule.Kind, evt.Kind) || (rule.Namespace != "" && rule.Namespace != evt.Namespace) {
			continue
		}

		if ok, err := e.evaluateRule(rule, evt, prev, currentVals); err != nil {
			e.logger.Warn("rule evaluation failed", "rule", rule.Name, "error", err)
		} else if ok {
			invs = append(invs, e.mapActions(rule, evt, currentVals)...)
		}
	}

	e.store.Set(evt.Key(), tracker.Snapshot{
		ResourceVersion: evt.ResourceVersionValue(),
		Values:          currentVals,
	})
	return invs, nil
}

func (e *Engine) mapActions(rule config.Rule, evt events.ResourceEvent, vals map[string]interface{}) []ActionInvocation {
	var results []ActionInvocation
	for _, id := range rule.Actions {
		if action, ok := e.cfg.Actions[id]; ok {
			results = append(results, ActionInvocation{
				RuleName: rule.Name,
				ActionID: id,
				Action:   action,
				Context: map[string]interface{}{
					"resource": evt.Object,
					"rule":     rule,
					"changes":  vals,
				},
			})
		}
	}
	return results
}

func (e *Engine) evaluateRule(rule config.Rule, evt events.ResourceEvent, prev tracker.Snapshot, collected map[string]interface{}) (bool, error) {
	if len(rule.Conditions) == 0 {
		return false, nil
	}

	isAny := strings.EqualFold(rule.Logic, "any")
	satisfiedAny := false

	for _, cond := range rule.Conditions {
		ok, val, err := e.evaluateCondition(cond, evt, prev)
		if err != nil {
			return false, err
		}
		if cond.Field != "" {
			collected[cond.Field] = val
		}
		if !isAny && !ok {
			return e.handleTimer(rule, evt, false), nil
		}
		if isAny && ok {
			satisfiedAny = true
		}
	}

	return e.handleTimer(rule, evt, !isAny || satisfiedAny), nil
}

func (e *Engine) handleTimer(rule config.Rule, evt events.ResourceEvent, met bool) bool {
	if rule.For <= 0 {
		return met
	}

	key := fmt.Sprintf("%s|%s", evt.Key(), rule.Name)
	e.timersMu.Lock()
	defer e.timersMu.Unlock()

	if !met {
		delete(e.timers, key)
		return false
	}

	if start, found := e.timers[key]; found {
		if time.Since(start) >= rule.For {
			delete(e.timers, key)
			return true
		}
		return false
	}

	e.timers[key] = time.Now()
	return false
}

func (e *Engine) evaluateCondition(cond config.Condition, evt events.ResourceEvent, prev tracker.Snapshot) (bool, interface{}, error) {
	val, err := extractValue(evt.Object, cond.Field)
	if err != nil {
		return false, nil, err
	}

	res := compareScalar(val, cond.Value)
	switch strings.ToLower(cond.Operator) {
	case "eq":
		return res == 0, val, nil
	case "ne":
		return res != 0, val, nil
	case "gt":
		return res > 0, val, nil
	case "lt":
		return res < 0, val, nil
	case "changed":
		return fmt.Sprintf("%v", prev.Values[cond.Field]) != fmt.Sprintf("%v", val), val, nil
	default:
		return false, val, fmt.Errorf("unsupported operator %s", cond.Operator)
	}
}

func compareScalar(a, b interface{}) int {
	af, aOk := toFloat(a)
	bf, bOk := toFloat(b)
	if aOk && bOk {
		if af < bf {
			return -1
		}
		if af > bf {
			return 1
		}
		return 0
	}
	as, bs := fmt.Sprintf("%v", a), fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
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
		if q, err := resource.ParseQuantity(t); err == nil {
			return q.AsApproximateFloat64(), true
		}
	}
	return 0, false
}

func extractValue(obj map[string]interface{}, path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	var current interface{} = obj
	for _, seg := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("segment %s not found", seg)
		}
		current, ok = m[strings.TrimSuffix(seg, "[*]")]
		if !ok {
			return nil, fmt.Errorf("field %s missing", seg)
		}
	}
	return current, nil
}
