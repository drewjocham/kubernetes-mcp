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

	"kube-watcher/pkg/convert"
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

func extractValue(jsonBytes []byte, path string) (interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	if len(jsonBytes) == 0 {
		return nil, fmt.Errorf("missing object data")
	}
	res := gjson.GetBytes(jsonBytes, path)
	if !res.Exists() {
		return nil, fmt.Errorf("field %s missing", path)
	}
	return res.Value(), nil
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
	e.compileCEL()
	return e
}

func (e *Engine) Evaluate(ctx context.Context, evt events.ResourceEvent) ([]ActionInvocation, error) {
	objJSON, err := json.Marshal(evt.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal resource: %w", err)
	}

	prev, _ := e.store.Get(evt.Key())
	currentVals := make(map[string]interface{})
	var invs []ActionInvocation

	for _, rule := range e.cfg.Rules {
		if !e.match(rule, evt) {
			continue
		}

		matched, err := e.execRule(rule, evt, prev, currentVals, objJSON)
		if err != nil {
			e.logger.Warn("rule evaluation failed", "rule", rule.Name, "error", err)
			continue
		}

		if e.checkTimer(rule, evt, matched) {
			invs = append(invs, e.mapActions(rule, evt, currentVals)...)
		}
	}

	e.persist(evt, currentVals)
	return invs, nil
}

func (e *Engine) execRule(rule config.Rule, evt events.ResourceEvent, prev tracker.Snapshot, col map[string]interface{}, raw []byte) (bool, error) {
	if prog, ok := e.celPrograms.Load(rule.Name); ok {
		return e.evalCEL(prog.(cel.Program), evt)
	}

	if len(rule.Conditions) == 0 {
		return false, nil
	}

	isAny := strings.EqualFold(rule.Logic, "any")
	satisfied := false

	for _, cond := range rule.Conditions {
		ok, val, err := e.evalCond(cond, prev, raw)
		if err != nil {
			return false, err
		}

		if cond.Field != "" {
			col[cond.Field] = val
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

func (e *Engine) evalCond(c config.Condition, prev tracker.Snapshot, raw []byte) (bool, interface{}, error) {
	res := gjson.GetBytes(raw, c.Field)
	if !res.Exists() {
		return false, nil, fmt.Errorf("field %s missing", c.Field)
	}
	val := res.Value()

	switch strings.ToLower(c.Operator) {
	case "changed":
		return fmt.Sprintf("%v", prev.Values[c.Field]) != fmt.Sprintf("%v", val), val, nil
	case "eq":
		return e.compare(val, c.Value) == 0, val, nil
	case "ne":
		return e.compare(val, c.Value) != 0, val, nil
	case "gt":
		return e.compare(val, c.Value) > 0, val, nil
	case "lt":
		return e.compare(val, c.Value) < 0, val, nil
	default:
		return false, val, fmt.Errorf("bad operator: %s", c.Operator)
	}
}

func (e *Engine) checkTimer(rule config.Rule, evt events.ResourceEvent, met bool) bool {
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

func (e *Engine) compare(a, b interface{}) int {
	af, aOk := e.toFloat(a)
	bf, bOk := e.toFloat(b)
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
	return strings.Compare(as, bs)
}

func (e *Engine) toFloat(v interface{}) (float64, bool) {
	return convert.ToFloat64(v)
}

func (e *Engine) evalCEL(prog cel.Program, evt events.ResourceEvent) (bool, error) {
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

func (e *Engine) match(r config.Rule, evt events.ResourceEvent) bool {
	return strings.EqualFold(r.Kind, evt.Kind) && (r.Namespace == "" || r.Namespace == evt.Namespace)
}

func (e *Engine) persist(evt events.ResourceEvent, vals map[string]interface{}) {
	snap := tracker.Snapshot{
		ResourceVersion: evt.ResourceVersionValue(),
		Values:          vals,
		Timestamp:       time.Now(),
	}
	e.store.Set(evt.Key(), snap)
	e.store.RecordHistory(evt.Key(), snap)
}

func (e *Engine) compileCEL() {
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
