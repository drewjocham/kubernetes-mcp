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
	celPrograms map[string]cel.Program
	timers      map[string]time.Time
	timersMu    sync.Mutex
}

func NewEngine(logger *slog.Logger, cfg *config.WatchConfig, store tracker.Store, celEnv *cel.Env) *Engine {
	e := &Engine{
		cfg:    cfg,
		store:  store,
		logger: logger,
		celEnv: celEnv,
		timers: make(map[string]time.Time),
	}
	e.compileCELPrograms()
	return e
}

func (e *Engine) Evaluate(ctx context.Context, evt events.ResourceEvent) ([]ActionInvocation, error) {
	prev, _ := e.store.Get(evt.Key())
	currentVals := make(map[string]interface{})
	var invs []ActionInvocation
	objJSON, err := json.Marshal(evt.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal resource: %w", err)
	}

	for _, rule := range e.cfg.Rules {
		if !e.matchRule(rule, evt) {
			continue
		}

		if ok, err := e.evaluateRule(rule, evt, prev, currentVals, objJSON); err != nil {
			e.logger.Warn("rule evaluation failed", "rule", rule.Name, "error", err)
		} else if ok {
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
	if prog, ok := e.celPrograms[rule.Name]; ok {
		okVal, err := e.evaluateExpression(prog, evt)
		if err != nil {
			return false, err
		}
		return e.handleTimer(rule, evt, okVal), nil
	}

	if len(rule.Conditions) == 0 {
		return false, nil
	}

	isAny := strings.EqualFold(rule.Logic, "any")
	satisfied := false

	for _, cond := range rule.Conditions {
		ok, val, err := e.evaluateCondition(cond, evt, prev, objJSON)
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
			return e.handleTimer(rule, evt, false), nil
		}
	}

	return e.handleTimer(rule, evt, satisfied), nil
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

	start, found := e.timers[key]
	if !found {
		e.timers[key] = time.Now()
		return false
	}

	if time.Since(start) >= rule.For {
		delete(e.timers, key)
		return true
	}
	return false
}

func (e *Engine) compileCELPrograms() {
	if e.celEnv == nil {
		return
	}
	e.celPrograms = make(map[string]cel.Program)
	for _, rule := range e.cfg.Rules {
		if rule.Expression == "" {
			continue
		}
		ast, issues := e.celEnv.Compile(rule.Expression)
		if issues != nil && issues.Err() != nil {
			e.logger.Warn("cel compile failed", "rule", rule.Name, "err", issues.Err())
			continue
		}
		prog, err := e.celEnv.Program(ast)
		if err != nil {
			continue
		}
		e.celPrograms[rule.Name] = prog
	}
}

func (e *Engine) evaluateExpression(prog cel.Program, evt events.ResourceEvent) (bool, error) {
	input := map[string]interface{}{
		"evt":  evt.Object,
		"kind": evt.Kind,
		"ns":   evt.Namespace,
		"name": evt.Name,
	}
	out, _, err := prog.Eval(input)
	if err != nil {
		return false, err
	}

	if b, ok := out.Value().(bool); ok {
		return b, nil
	}
	if b, ok := out.Value().(types.Bool); ok {
		return bool(b), nil
	}
	return false, fmt.Errorf("non-bool return")
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

func (e *Engine) evaluateCondition(cond config.Condition, evt events.ResourceEvent, prev tracker.Snapshot, objJSON []byte) (bool, interface{}, error) {
	val, err := extractValue(objJSON, cond.Field)
	if err != nil {
		return false, nil, err
	}

	switch strings.ToLower(cond.Operator) {
	case "changed":
		return fmt.Sprintf("%v", prev.Values[cond.Field]) != fmt.Sprintf("%v", val), val, nil
	case "eq":
		return compareScalar(val, cond.Value) == 0, val, nil
	case "ne":
		return compareScalar(val, cond.Value) != 0, val, nil
	case "gt":
		return compareScalar(val, cond.Value) > 0, val, nil
	case "lt":
		return compareScalar(val, cond.Value) < 0, val, nil
	default:
		return false, val, fmt.Errorf("bad operator: %s", cond.Operator)
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

func extractValue(jsonBytes []byte, path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	if len(jsonBytes) == 0 {
		return nil, fmt.Errorf("missing object data")
	}
	result := gjson.GetBytes(jsonBytes, path)
	if !result.Exists() {
		return nil, fmt.Errorf("field %s missing", path)
	}
	return result.Value(), nil
}

func init() {
	gjson.AddModifier("k8s_sum", func(jsonStr, _ string) string {
		result := gjson.Parse(jsonStr)
		var total resource.Quantity
		result.ForEach(func(_, value gjson.Result) bool {
			if q, err := resource.ParseQuantity(value.String()); err == nil {
				total.Add(q)
			}
			return true
		})
		return fmt.Sprintf("%d", total.MilliValue())
	})
}
