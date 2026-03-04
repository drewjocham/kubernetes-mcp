package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/tidwall/gjson"
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
	Event    events.ResourceEvent
}

type Engine struct {
	cfg         *config.WatchConfig
	store       tracker.Store
	logger      *slog.Logger
	celEnv      *cel.Env
	celPrograms sync.Map
	timers      sync.Map
}

func NewEngine(logger *slog.Logger, cfg *config.WatchConfig, store tracker.Store, celEnv *cel.Env) *Engine {
	e := &Engine{
		cfg:    cfg,
		store:  store,
		logger: logger,
		celEnv: celEnv,
	}
	e.compileCELPrograms()
	return e
}

func (e *Engine) Evaluate(ctx context.Context, evt events.ResourceEvent) ([]ActionInvocation, error) {
	objJSON, err := json.Marshal(evt.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	prev, _ := e.store.Get(evt.Key())
	currentVals := make(map[string]interface{})
	var invs []ActionInvocation

	for _, rule := range e.cfg.Rules {
		if !e.matchRule(rule, evt) {
			continue
		}

		ok, err := e.evaluateRule(rule, evt, prev, currentVals, objJSON)
		if err != nil {
			e.logger.Warn("rule_fail", "rule", rule.Name, "err", err)
			continue
		}

		if ok {
			invs = append(invs, e.mapActions(rule, evt, currentVals)...)
		}
	}

	snap := tracker.Snapshot{
		ResourceVersion: evt.ResourceVersionValue(),
		Values:          currentVals,
		Timestamp:       time.Now(),
	}
	e.store.Set(evt.Key(), snap)
	e.store.RecordHistory(evt.Key(), snap)
	return invs, nil
}

func (e *Engine) matchRule(r config.Rule, evt events.ResourceEvent) bool {
	return strings.EqualFold(r.Kind, evt.Kind) && (r.Namespace == "" || r.Namespace == evt.Namespace)
}

func (e *Engine) evaluateRule(rule config.Rule, evt events.ResourceEvent, prev tracker.Snapshot, collected map[string]interface{}, objJSON []byte) (bool, error) {
	var met bool
	var err error

	if prog, ok := e.celPrograms.Load(rule.Name); ok {
		met, err = e.evaluateExpression(prog.(cel.Program), evt)
	} else if len(rule.Conditions) > 0 {
		met, err = e.evaluateStructured(rule, evt, prev, collected, objJSON)
	}

	if err != nil {
		return false, err
	}

	return e.handleTimer(rule, evt, met), nil
}

func (e *Engine) evaluateStructured(rule config.Rule, evt events.ResourceEvent, prev tracker.Snapshot, collected map[string]interface{}, objJSON []byte) (bool, error) {
	isAny := strings.EqualFold(rule.Logic, "any")
	satisfied := false

	for _, cond := range rule.Conditions {
		ok, val, err := e.evaluateCondition(cond, prev, objJSON)
		if err != nil {
			return false, err
		}
		if cond.Field != "" {
			collected[cond.Field] = val
		}

		if ok {
			satisfied = true
			if isAny {
				break
			}
		} else if !isAny {
			return false, nil
		}
	}
	return satisfied, nil
}

func (e *Engine) handleTimer(rule config.Rule, evt events.ResourceEvent, met bool) bool {
	if rule.For <= 0 {
		return met
	}

	key := fmt.Sprintf("%s|%s", evt.Key(), rule.Name)
	if !met {
		e.timers.Delete(key)
		return false
	}

	now := time.Now()
	start, loaded := e.timers.LoadOrStore(key, now)
	if !loaded {
		return false
	}

	if now.Sub(start.(time.Time)) >= rule.For {
		e.timers.Delete(key)
		return true
	}
	return false
}

func (e *Engine) compileCELPrograms() {
	if e.celEnv == nil {
		return
	}
	for _, rule := range e.cfg.Rules {
		if rule.Expression == "" {
			continue
		}
		ast, issues := e.celEnv.Compile(rule.Expression)
		if issues != nil && issues.Err() != nil {
			continue
		}
		prog, err := e.celEnv.Program(ast)
		if err == nil {
			e.celPrograms.Store(rule.Name, prog)
		}
	}
}

func (e *Engine) evaluateExpression(prog cel.Program, evt events.ResourceEvent) (bool, error) {
	out, _, err := prog.Eval(map[string]interface{}{
		"evt":  evt.Object,
		"kind": evt.Kind,
		"ns":   evt.Namespace,
		"name": evt.Name,
	})
	if err != nil {
		return false, err
	}
	if b, ok := out.Value().(bool); ok {
		return b, nil
	}
	if b, ok := out.Value().(types.Bool); ok {
		return bool(b), nil
	}
	return false, fmt.Errorf("non-bool")
}

func (e *Engine) mapActions(rule config.Rule, evt events.ResourceEvent, vals map[string]interface{}) []ActionInvocation {
	var res []ActionInvocation
	for _, id := range rule.Actions {
		if action, ok := e.cfg.Actions[id]; ok {
			res = append(res, ActionInvocation{
				RuleName: rule.Name,
				ActionID: id,
				Action:   action,
				Context: map[string]interface{}{
					"resource": evt.Object,
					"rule":     rule,
					"changes":  vals,
				},
				Event: evt,
			})
		}
	}
	return res
}

func (e *Engine) evaluateCondition(cond config.Condition, prev tracker.Snapshot, objJSON []byte) (bool, interface{}, error) {
	val, err := extractValue(objJSON, cond.Field)
	if err != nil {
		return false, nil, err
	}

	op := strings.ToLower(cond.Operator)
	switch op {
	case "changed":
		prevVal := prev.Values[cond.Field]
		return !compareEqual(val, prevVal), val, nil
	case "eq":
		return compareScalar(val, cond.Value) == 0, val, nil
	case "ne":
		return compareScalar(val, cond.Value) != 0, val, nil
	case "gt":
		return compareScalar(val, cond.Value) > 0, val, nil
	case "lt":
		return compareScalar(val, cond.Value) < 0, val, nil
	default:
		return false, val, fmt.Errorf("unsupported operator: %s", op)
	}
}

func compareEqual(a, b interface{}) bool {
	if a == b {
		return true
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case float32:
		return float64(t), true
	case string:
		if q, err := resource.ParseQuantity(t); err == nil {
			return q.AsApproximateFloat64(), true
		}
	}
	return 0, false
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

func extractValue(jsonBytes []byte, path string) (interface{}, error) {
	result := gjson.GetBytes(jsonBytes, path)
	if !result.Exists() {
		return nil, fmt.Errorf("missing")
	}
	return result.Value(), nil
}

func init() {
	gjson.AddModifier("k8s_sum", func(jsonStr, _ string) string {
		var total resource.Quantity
		gjson.Parse(jsonStr).ForEach(func(_, value gjson.Result) bool {
			if q, err := resource.ParseQuantity(value.String()); err == nil {
				total.Add(q)
			}
			return true
		})
		return fmt.Sprintf("%d", total.MilliValue())
	})
}
