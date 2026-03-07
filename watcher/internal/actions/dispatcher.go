package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/panjf2000/ants/v2"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/rules"
)

const (
	defaultOzAPIURL    = "https://app.warp.dev/api/v1/agent/run"
	defaultOzAPIKeyEnv = "OZ_API_KEY"
)

type Dispatcher struct {
	logger  *slog.Logger
	actions map[string]config.Action
	pool    *ants.PoolWithFunc
	mu      sync.Mutex
	lastRun map[string]time.Time
	client  *http.Client
	getEnv  func(string) string
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
		client:  http.DefaultClient,
		getEnv:  os.Getenv,
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
		d.handleNotification(inv)
	case "oz-agent":
		d.handleOzAgent(inv)
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

func (d *Dispatcher) handleNotification(inv rules.ActionInvocation) {
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

	// Google Chat webhooks expect a message format like {"text": "..."}
	payload := map[string]string{"text": msg}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		d.logger.Warn("failed to marshal notification payload",
			"error", err, "rule", inv.RuleName)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		d.logger.Warn("failed to create notification request",
			"error", err, "rule", inv.RuleName)
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := d.client.Do(req)
	if err != nil {
		d.logger.Warn("failed to send notification",
			"error", err, "rule", inv.RuleName)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		d.logger.Warn("notification webhook returned non-200 status",
			"status", resp.Status, "rule", inv.RuleName)
		return
	}

	d.logger.Info("rule action executed",
		"rule", inv.RuleName, "action", inv.ActionID, "type", "notification")
}

func (d *Dispatcher) handleOzAgent(inv rules.ActionInvocation) {
	envVarName := inv.Action.Config["api_key_env"]
	if envVarName == "" {
		envVarName = defaultOzAPIKeyEnv
	}
	apiKey := d.getEnv(envVarName)
	if apiKey == "" {
		d.logger.Warn("oz-agent action missing API key",
			"env_var", envVarName, "rule", inv.RuleName, "action", inv.ActionID)
		return
	}

	envID := inv.Action.Config["environment_id"]
	if envID == "" {
		d.logger.Warn("oz-agent action missing environment_id",
			"rule", inv.RuleName, "action", inv.ActionID)
		return
	}

	prompt, err := d.renderTemplate(inv.Action.Template, map[string]interface{}{
		"RuleName":  inv.RuleName,
		"ActionID":  inv.ActionID,
		"Context":   inv.Context,
		"Event":     inv.Event,
		"Kind":      inv.Event.Kind,
		"Namespace": inv.Event.Namespace,
		"Name":      inv.Event.Name,
	})
	if err != nil {
		d.logger.Warn("oz-agent template failure",
			"error", err, "rule", inv.RuleName)
		return
	}

	apiURL := inv.Action.Config["api_url"]
	if apiURL == "" {
		apiURL = defaultOzAPIURL
	}

	body := map[string]interface{}{
		"prompt": prompt,
		"config": map[string]string{
			"environment_id": envID,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		d.logger.Warn("oz-agent failed to marshal request",
			"error", err, "rule", inv.RuleName)
		return
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		d.logger.Warn("oz-agent failed to create request",
			"error", err, "rule", inv.RuleName)
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		d.logger.Warn("oz-agent API call failed",
			"error", err, "rule", inv.RuleName, "action", inv.ActionID)
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		d.logger.Warn("oz-agent API returned non-success status",
			"status", resp.Status, "body", string(respBody),
			"rule", inv.RuleName, "action", inv.ActionID)
		return
	}

	var result map[string]interface{}
	runID := "unknown"
	if err := json.Unmarshal(respBody, &result); err == nil {
		if id, ok := result["id"].(string); ok {
			runID = id
		}
	}

	d.logger.Info("oz-agent spawned",
		"rule", inv.RuleName, "action", inv.ActionID,
		"run_id", runID, "environment_id", envID)
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
