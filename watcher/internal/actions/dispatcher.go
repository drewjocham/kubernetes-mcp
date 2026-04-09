package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"text/template"
	"time"

	"github.com/panjf2000/ants/v2"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/rules"
)

type Dispatcher struct {
	logger  *slog.Logger
	actions map[string]config.Action
	pool    *ants.PoolWithFunc
	mu      sync.Mutex
	lastRun map[string]time.Time
}

type dispatchTask struct {
	ctx        context.Context
	invocation rules.ActionInvocation
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

	pool, err := ants.NewPoolWithFunc(size, func(i interface{}) {
		d.execute(i.(dispatchTask))
	})
	if err != nil {
		return nil, err
	}

	d.pool = pool
	return d, nil
}

func (d *Dispatcher) Dispatch(ctx context.Context, inv rules.ActionInvocation) error {
	return d.pool.Invoke(dispatchTask{ctx: ctx, invocation: inv})
}

func (d *Dispatcher) execute(task dispatchTask) {
	inv := task.invocation
	if !d.isAllowed(inv) {
		return
	}

	switch inv.Action.Type {
	case "log":
		d.handleLog(inv)
	case "notification":
		d.handleNotification(task, inv)
	case "webhook":
		d.handleWebhook(task, inv)
	default:
		d.logger.Info("action executed",
			"type", inv.Action.Type, "rule", inv.RuleName)
	}
}

func (d *Dispatcher) handleLog(inv rules.ActionInvocation) {
	msg, err := d.renderTemplate(inv.Action.Template, inv.Context)
	if err != nil {
		d.logger.Warn("action template failure",
			"error", err, "rule", inv.RuleName)
		return
	}
	d.logger.Info("rule action",
		"rule", inv.RuleName, "action", inv.ActionID, "message", msg)
}

func (d *Dispatcher) handleNotification(task dispatchTask, inv rules.ActionInvocation) {
	url, ok := inv.Action.Config["url"]
	if !ok || url == "" {
		d.logger.Warn("action notification missing url",
			"rule", inv.RuleName, "action", inv.ActionID)
		return
	}

	msg, err := d.renderTemplate(inv.Action.Template, inv.Context)
	if err != nil {
		d.logger.Warn("action template failure",
			"error", err, "rule", inv.RuleName)
		return
	}

	payload := map[string]string{"text": msg}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		d.logger.Warn("failed to marshal notification payload",
			"error", err, "rule", inv.RuleName)
		return
	}

	if err := d.postJSON(task.ctx, url, jsonPayload, inv); err != nil {
		d.logger.Warn("failed to send notification", "error", err, "rule", inv.RuleName)
		return
	}

	d.logger.Info("rule action executed",
		"rule", inv.RuleName, "action", inv.ActionID, "type", "notification")
}
func (d *Dispatcher) handleWebhook(task dispatchTask, inv rules.ActionInvocation) {
	url, ok := inv.Action.Config["url"]
	if !ok || url == "" {
		d.logger.Warn("action webhook missing url", "rule", inv.RuleName, "action", inv.ActionID)
		return
	}

	var payload []byte
	var err error

	templateStr := inv.Action.Template
	if templateStr != "" {
		raw, renderErr := d.renderTemplate(templateStr, map[string]interface{}{
			"RuleName": inv.RuleName,
			"ActionID": inv.ActionID,
			"Context":  inv.Context,
			"Event":    inv.Event,
		})
		if renderErr != nil {
			d.logger.Warn("action webhook template failure", "error", renderErr, "rule", inv.RuleName)
			return
		}
		payload = []byte(raw)
	} else {
		payload, err = json.Marshal(map[string]interface{}{
			"ruleName": inv.RuleName,
			"actionID": inv.ActionID,
			"context":  inv.Context,
			"event":    inv.Event,
		})
		if err != nil {
			d.logger.Warn("failed to marshal webhook payload", "error", err, "rule", inv.RuleName)
			return
		}
	}

	if err := d.postJSON(task.ctx, url, payload, inv); err != nil {
		d.logger.Warn("failed to send webhook", "error", err, "rule", inv.RuleName)
		return
	}

	d.logger.Info("rule action executed",
		"rule", inv.RuleName, "action", inv.ActionID, "type", "webhook")
}

func (d *Dispatcher) postJSON(ctx context.Context, url string, payload []byte, inv rules.ActionInvocation) error {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("X-Kube-Watcher-Rule", inv.RuleName)
	req.Header.Set("X-Kube-Watcher-Action", inv.ActionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		return fmt.Errorf("webhook returned non-2xx status: %s", resp.Status)
	}

	return nil
}

func (d *Dispatcher) renderTemplate(tmplStr string, data interface{}) (string, error) {
	tmpl, err := template.New("action").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (d *Dispatcher) isAllowed(inv rules.ActionInvocation) bool {
	throttle := inv.Action.Throttle
	window, limit := time.Minute, throttle.MaxPerMinute

	if limit == 0 && throttle.MaxPerHour > 0 {
		window, limit = time.Hour, throttle.MaxPerHour
	}

	if limit == 0 {
		return true
	}

	key := fmt.Sprintf("%s:%s", inv.RuleName, inv.ActionID)
	d.mu.Lock()
	defer d.mu.Unlock()

	if time.Since(d.lastRun[key]) < window/time.Duration(limit) {
		return false
	}

	d.lastRun[key] = time.Now()
	return true
}

func (d *Dispatcher) Close() {
	d.pool.Release()
}
