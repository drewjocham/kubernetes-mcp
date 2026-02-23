package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	gochi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/riandyrn/otelchi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"kube-watcher/mcp/server"
)

const (
	prefix                = "/v1"
	defaultRootMessage    = "kube-watcher MCP API ("
	pathTools             = prefix + "/tools"
	pathToolExecute       = prefix + "/tools/{tool}"
	pathAlerts            = prefix + "/alerts"
	pathHistory           = prefix + "/history"
	pathHealth            = "/healthz"
	pathReady             = "/readyz"
	traceAttrToolName     = "tool.name"
	traceAttrRoutePattern = "http.route"
	logMsgPanicRecovered  = "panic recovered"
	logKeyError           = "error"
	logKeyStack           = "stack"
	defaultHistoryWindow  = 72 * time.Hour
	contentTypeJSON       = "application/json"
	errMessageInternal    = "internal server error"
	statusOK              = "ok"
	defaultServiceName    = "kube-watcher-api"
	queryWindow           = "window"
	routeParamTool        = "tool"
)

var (
	ErrServerNil = errors.New("api: MCP server is required")
)

type Checker interface {
	Check(context.Context) error
}

type CheckerFunc func(context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

type Config struct {
	Server           *server.MCPServer
	Logger           *slog.Logger
	ServiceName      string
	LivenessChecker  Checker
	ReadinessChecker Checker
	Tracer           trace.Tracer
	Version          string
}

type API struct {
	server           *server.MCPServer
	logger           *slog.Logger
	serviceName      string
	liveness         Checker
	readiness        Checker
	tracer           trace.Tracer
	version          string
	defaultResponder string
}

func New(cfg Config) (*API, error) {
	if cfg.Server == nil {
		return nil, ErrServerNil
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = defaultServiceName
	}
	if cfg.Tracer == nil {
		cfg.Tracer = otel.Tracer(cfg.ServiceName)
	}
	api := &API{
		server:           cfg.Server,
		logger:           cfg.Logger,
		serviceName:      cfg.ServiceName,
		liveness:         cfg.LivenessChecker,
		readiness:        cfg.ReadinessChecker,
		tracer:           cfg.Tracer,
		version:          cfg.Version,
		defaultResponder: defaultRootMessage + cfg.ServiceName + ")",
	}
	return api, nil
}

func (a *API) Routes() http.Handler {
	r := gochi.NewRouter()
	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(otelchi.Middleware(a.serviceName, otelchi.WithChiRoutes(r)))

	r.Get("/", a.wrap("root", a.handleRoot))
	r.Get(pathHealth, a.wrap("healthz", a.handleHealth))
	r.Get(pathReady, a.wrap("readyz", a.handleReady))

	r.Get(prefix+"/status", a.wrap("status", a.handleStatus))
	r.Get(pathTools, a.wrap("list-tools", a.handleListTools))
	r.Post(pathToolExecute, a.wrap("exec-tool", a.handleExecuteTool))
	r.Get(pathAlerts, a.wrap("alerts", a.handleAlerts))
	r.Get(pathHistory, a.wrap("history", a.handleHistory))

	return r
}

type handlerFunc func(context.Context, http.ResponseWriter, *http.Request) error

func (a *API) wrap(spanName string, fn handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.execute(w, r, spanName, fn)
	}
}

func (a *API) handleRoot(ctx context.Context, w http.ResponseWriter, _ *http.Request) error {
	_, err := w.Write([]byte(a.defaultResponder))
	return err
}

func (a *API) handleStatus(ctx context.Context, w http.ResponseWriter, _ *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": a.serviceName,
		"version": a.version,
		"status":  statusOK,
	})
}

func (a *API) handleHealth(ctx context.Context, w http.ResponseWriter, _ *http.Request) error {
	if a.liveness != nil {
		if err := a.liveness.Check(ctx); err != nil {
			return newHTTPError(http.StatusServiceUnavailable, "liveness probe failed", err)
		}
	}
	return writeJSON(w, http.StatusOK, map[string]string{"status": statusOK})
}

func (a *API) handleReady(ctx context.Context, w http.ResponseWriter, _ *http.Request) error {
	if a.readiness != nil {
		if err := a.readiness.Check(ctx); err != nil {
			return newHTTPError(http.StatusServiceUnavailable, "readiness probe failed", err)
		}
	}
	return writeJSON(w, http.StatusOK, map[string]string{"status": statusOK})
}

func (a *API) handleListTools(ctx context.Context, w http.ResponseWriter, _ *http.Request) error {
	tools := a.server.ToolSummaries()
	return writeJSON(w, http.StatusOK, map[string]interface{}{"tools": tools})
}

func (a *API) handleExecuteTool(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	name := gochi.URLParam(r, routeParamTool)
	if name == "" {
		return newHTTPError(http.StatusBadRequest, "tool name required", nil)
	}

	args, err := decodeJSONBody(r)
	if err != nil {
		return err
	}

	result, execErr := a.server.ExecuteTool(ctx, name, args)
	if execErr != nil {
		if errors.Is(execErr, server.ErrToolNotFound) {
			return newHTTPError(http.StatusNotFound, "tool not found", execErr)
		}
		return execErr
	}
	return writeJSON(w, http.StatusOK, result)
}

func (a *API) handleAlerts(ctx context.Context, w http.ResponseWriter, _ *http.Request) error {
	alerts := a.server.AlertsSnapshot()
	return writeJSON(w, http.StatusOK, map[string]interface{}{"alerts": alerts})
}

func (a *API) handleHistory(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	window := defaultHistoryWindow
	if q := r.URL.Query().Get(queryWindow); q != "" {
		if parsed, err := time.ParseDuration(q); err == nil {
			window = parsed
		}
	}
	records, err := a.server.IncidentHistory(ctx, window)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]interface{}{
		"window":  window.String(),
		"records": records,
	})
}

func (a *API) execute(w http.ResponseWriter, r *http.Request, spanName string, fn handlerFunc) {
	ctx := r.Context()
	ctx, span := a.tracer.Start(ctx, spanName, trace.WithAttributes(a.extractTraceAttrs(r)...))
	defer span.End()

	defer func() {
		if rec := recover(); rec != nil {
			a.logger.ErrorContext(ctx, logMsgPanicRecovered, logKeyError, rec, logKeyStack, string(debug.Stack()))
			writeError(w, newHTTPError(http.StatusInternalServerError, errMessageInternal, errors.New("panic recovered")))
			span.RecordError(errors.New("panic recovered"))
		}
	}()

	if err := fn(ctx, w, r.WithContext(ctx)); err != nil {
		span.RecordError(err)
		writeError(w, err)
		if !errors.Is(err, context.Canceled) {
			a.logger.ErrorContext(ctx, "request failed", logKeyError, err)
		}
	}
}

func (a *API) extractTraceAttrs(r *http.Request) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(traceAttrRoutePattern, r.URL.Path),
	}
	if tool := gochi.URLParam(r, routeParamTool); tool != "" {
		attrs = append(attrs, attribute.String(traceAttrToolName, tool))
	}
	return attrs
}

func decodeJSONBody(r *http.Request) (map[string]interface{}, error) {
	if r.Body == nil {
		return map[string]interface{}{}, nil
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, newHTTPError(http.StatusBadRequest, "unable to read request body", err)
	}
	if len(data) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, newHTTPError(http.StatusBadRequest, "invalid JSON payload", err)
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) error {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(payload)
}

type httpError struct {
	status  int
	message string
	err     error
}

func (e *httpError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.message
}

func (e *httpError) Unwrap() error {
	return e.err
}

func newHTTPError(status int, message string, err error) *httpError {
	return &httpError{status: status, message: message, err: err}
}

func writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	resp := map[string]string{"error": errMessageInternal}
	status := http.StatusInternalServerError
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		status = httpErr.status
		if httpErr.message != "" {
			resp["error"] = httpErr.message
		}
	} else if err.Error() != "" {
		resp["error"] = err.Error()
	}
	_ = writeJSON(w, status, resp)
}
