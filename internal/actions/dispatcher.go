package actions

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/panjf2000/ants/v2"

	"kube-watcher/internal/config"
	"kube-watcher/internal/rules"
)

type Dispatcher struct {
	logger  *slog.Logger
	actions map[string]config.Action
	pool    *ants.PoolWithFunc
	mu      sync.Mutex
	lastRun map[string]time.Time
}

func NewDispatcher(logger *slog.Logger, actions map[string]config.Action, size int) (*Dispatcher, error) {
	if size <= 0 {
		size = 32
	}
	d := &Dispatcher{
		logger:  logger,
		actions: actions,
		lastRun: make(map[string]time.Time),
	}
	pool, err := ants.NewPoolWithFunc(size, d.execute)
	if err != nil {
		return nil, err
	}
	d.pool = pool
	return d, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, inv rules.ActionInvocation) error {
	return d.pool.Invoke(dispatchTask{ctx: ctx, invocation: inv})
}

type dispatchTask struct {
	ctx        context.Context
	invocation rules.ActionInvocation
}

func (d *Dispatcher) execute(payload interface{}) {
	task := payload.(dispatchTask)
	action := task.invocation.Action
	if !d.allow(action, task.invocation) {
		return
	}
	switch action.Type {
	case "log":
		d.runLog(task)
	default:
		d.logger.Info("action executed", "type", action.Type, "rule", task.invocation.RuleName)
	}
}

func (d *Dispatcher) runLog(task dispatchTask) {
	tmpl, err := template.New("log").Parse(task.invocation.Action.Template)
	if err != nil {
		d.logger.Warn("template parse failed", "error", err)
		return
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, task.invocation.Context); err != nil {
		d.logger.Warn("template exec failed", "error", err)
		return
	}
	d.logger.Info("rule action", "rule", task.invocation.RuleName, "action", task.invocation.ActionID, "message", buf.String())
}

func (d *Dispatcher) allow(action config.Action, inv rules.ActionInvocation) bool {
	window := time.Minute
	limit := action.Throttle.MaxPerMinute
	if limit == 0 && action.Throttle.MaxPerHour > 0 {
		window = time.Hour
		limit = action.Throttle.MaxPerHour
	}
	if limit == 0 {
		return true
	}
	key := inv.RuleName + ":" + inv.ActionID
	d.mu.Lock()
	defer d.mu.Unlock()
	last, ok := d.lastRun[key]
	if !ok || time.Since(last) >= window/time.Duration(limit) {
		d.lastRun[key] = time.Now()
		return true
	}
	return false
}
