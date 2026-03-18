package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcpapi "kube-watcher/mcp/api"
	"kube-watcher/mcp/monitoring/history"
	mcpserver "kube-watcher/mcp/server"
	"kube-watcher/pkg/kube"
	kwatch "kube-watcher/pkg/kube/watch"

	"kube-watcher/watcher/internal/actions"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/pipeline"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/tracker"

	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
)

type uiBridgeSignal struct {
	AlertPayload map[string]any
	MCPPayload   map[string]any
}

type mcpHTTPBridgeSignal struct {
	AlertPayload map[string]any
	MCPResponse  map[string]any
}

type mcpStubK8sClient struct {
	pods []kube.PodInfo
}

func (s *mcpStubK8sClient) GetNodes(context.Context) ([]kube.NodeInfo, error) { return nil, nil }
func (s *mcpStubK8sClient) GetNode(context.Context, string) (*kube.NodeInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetPods(_ context.Context, _ string) ([]kube.PodInfo, error) {
	return s.pods, nil
}
func (s *mcpStubK8sClient) GetPodsAllNamespaces(context.Context) ([]kube.PodInfo, error) {
	return s.pods, nil
}
func (s *mcpStubK8sClient) GetPod(context.Context, string, string) (*kube.PodInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetPodLogs(context.Context, string, string, string, int64, int64, bool) (string, error) {
	return "", nil
}
func (s *mcpStubK8sClient) GetServices(context.Context, string) ([]kube.ServiceInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetServicesAllNamespaces(context.Context) ([]kube.ServiceInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetEvents(context.Context, string) ([]kube.EventInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetEventsAllNamespaces(context.Context) ([]kube.EventInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetNamespaces(context.Context) ([]string, error) { return nil, nil }
func (s *mcpStubK8sClient) GetResourceQuotas(context.Context, string) ([]kube.ResourceQuotaInfo, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) GetClusterInfo(context.Context) (*kube.ClusterInfo, error) {
	return &kube.ClusterInfo{Version: "v1.28"}, nil
}
func (s *mcpStubK8sClient) GetResource(context.Context, string, string) ([]runtime.Object, error) {
	return nil, nil
}
func (s *mcpStubK8sClient) HealthCheck(context.Context) error { return nil }
func (s *mcpStubK8sClient) GetRawInterface() clientset.Interface {
	return nil
}

func TestChannelEngineToUIAndRealMCPAPIIntegration(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	k8sClient := &mcpStubK8sClient{
		pods: []kube.PodInfo{
			{Name: "checkout-api-7ff98f4bcf-6jwkq", Namespace: "payments", Phase: "Running", Status: "Running", RestartCount: 7},
		},
	}

	historyStore, err := history.NewStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = historyStore.Close() })

	watcherManager := kwatch.NewManager(k8sClient, logger, 30*time.Second)
	mcpSrv, err := mcpserver.NewMCPServer(logger, mcpserver.Config{
		Version:      "test",
		GitCommit:    "test",
		BuildDate:    "now",
		K8sClient:    k8sClient,
		HistoryStore: historyStore,
		Watcher:      watcherManager,
	})
	require.NoError(t, err)

	apiInstance, err := mcpapi.New(mcpapi.Config{
		Server:      mcpSrv,
		Logger:      logger,
		ServiceName: "mcp-api-integration-test",
		Version:     "test",
	})
	require.NoError(t, err)

	mcpHTTPServer := httptest.NewServer(apiInstance.Routes())
	defer mcpHTTPServer.Close()

	signalCh := make(chan mcpHTTPBridgeSignal, 1)
	uiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/alerts/ingest" {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = r.Body.Close() }()

		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var alertPayload map[string]any
		require.NoError(t, json.Unmarshal(raw, &alertPayload))

		mcpBody, err := json.Marshal(map[string]any{
			"namespace":        alertPayload["namespace"],
			"problematic_only": true,
		})
		require.NoError(t, err)

		resp, err := http.Post(mcpHTTPServer.URL+"/v1/tools/get_pod_resources", "application/json", bytes.NewReader(mcpBody))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var mcpResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&mcpResp))

		signalCh <- mcpHTTPBridgeSignal{
			AlertPayload: alertPayload,
			MCPResponse:  mcpResp,
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer uiServer.Close()

	logBuffer := &safeBuffer{}
	engineLogger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:    "pod_failure_to_real_mcp_api",
				Kind:    "Pod",
				Logic:   "all",
				Actions: []string{"dashboard_webhook"},
				Conditions: []config.Condition{
					{Field: "restart_delta", Operator: "gt", Value: 0},
				},
			},
		},
		Actions: map[string]config.Action{
			"dashboard_webhook": {
				Type: "webhook",
				Config: map[string]string{
					"url": uiServer.URL + "/api/alerts/ingest",
				},
				Template: `{
  "kind":"CrashLoopBackOff",
  "cluster":"integration-cluster",
  "namespace":"{{ .Event.Namespace }}",
  "pod":"{{ .Event.Name }}",
  "ruleName":"{{ .RuleName }}",
  "source":"kube-watcher"
}`,
			},
		},
		Settings: config.Settings{QueueDepth: 8},
	}

	store := tracker.NewMemoryStore()
	t.Cleanup(func() { _ = store.Close() })
	engine := rules.NewEngine(engineLogger, cfg, store, nil)

	dispatcher, err := actions.NewDispatcher(engineLogger, cfg.Actions, 4)
	require.NoError(t, err)
	t.Cleanup(dispatcher.Close)

	src := eventSource{
		events: []events.ResourceEvent{
			{
				Kind:      "Pod",
				Namespace: "payments",
				Name:      "checkout-api-7ff98f4bcf-6jwkq",
				Object: map[string]any{
					"metadata":      map[string]any{"name": "checkout-api-7ff98f4bcf-6jwkq"},
					"restart_delta": 1,
				},
			},
		},
	}

	pipe := pipeline.New(
		engineLogger,
		src,
		pipeline.NewRuleAwareFilter(cfg),
		nil,
		engine,
		dispatcher,
		store,
		nil,
		cfg.Settings.QueueDepth,
		2,
		10,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pipe.Start(ctx)

	select {
	case signal := <-signalCh:
		assert.Equal(t, "CrashLoopBackOff", signal.AlertPayload["kind"])
		assert.Equal(t, "payments", signal.AlertPayload["namespace"])
		assert.Equal(t, "checkout-api-7ff98f4bcf-6jwkq", signal.AlertPayload["pod"])
		assert.Equal(t, "pod_failure_to_real_mcp_api", signal.AlertPayload["ruleName"])

		summary, ok := signal.MCPResponse["summary"].(map[string]any)
		require.True(t, ok, "mcp response should include summary")
		assert.EqualValues(t, 1, summary["total"])
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for engine->ui->real mcp api chain. logs: %s", logBuffer.String())
	}
}

func TestChannelEngineToUIAndMCPIntegration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		alertKind     string
		waitingReason string
	}{
		{
			name:          "CrashLoopBackOff alert reaches UI and triggers MCP enrichment",
			alertKind:     "CrashLoopBackOff",
			waitingReason: "CrashLoopBackOff",
		},
		{
			name:          "OOMKilled alert reaches UI and triggers MCP enrichment",
			alertKind:     "OOMKilled",
			waitingReason: "OOMKilled",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mcpPayloadCh := make(chan map[string]any, 1)
			mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer func() { _ = r.Body.Close() }()
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var payload map[string]any
				require.NoError(t, json.Unmarshal(raw, &payload))
				mcpPayloadCh <- payload

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"lastLogLines":["line-a","line-b"],"describeOutput":"ok","clusterEvents":["evt-1"]}`))
			}))
			defer mcpServer.Close()

			signalCh := make(chan uiBridgeSignal, 1)
			uiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/alerts/ingest" {
					http.NotFound(w, r)
					return
				}

				defer func() { _ = r.Body.Close() }()
				raw, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var alertPayload map[string]any
				require.NoError(t, json.Unmarshal(raw, &alertPayload))

				mcpReq := map[string]any{
					"namespace": alertPayload["namespace"],
					"pod":       alertPayload["pod"],
					"kind":      alertPayload["kind"],
					"lines":     50,
				}

				reqBody, err := json.Marshal(mcpReq)
				require.NoError(t, err)
				resp, err := http.Post(mcpServer.URL+"/diagnostics", "application/json", bytes.NewReader(reqBody))
				require.NoError(t, err)
				_ = resp.Body.Close()

				signalCh <- uiBridgeSignal{
					AlertPayload: alertPayload,
					MCPPayload:   <-mcpPayloadCh,
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer uiServer.Close()

			logBuffer := &safeBuffer{}
			logger := slog.New(slog.NewTextHandler(logBuffer, &slog.HandlerOptions{Level: slog.LevelInfo}))

			cfg := &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:    "pod_failure_to_ui",
						Kind:    "Pod",
						Logic:   "all",
						Actions: []string{"dashboard_webhook"},
						Conditions: []config.Condition{
							{Field: "restart_delta", Operator: "gt", Value: 0},
						},
					},
				},
				Actions: map[string]config.Action{
					"dashboard_webhook": {
						Type: "webhook",
						Config: map[string]string{
							"url": uiServer.URL + "/api/alerts/ingest",
						},
						Template: `{
  "kind":"` + tc.alertKind + `",
  "cluster":"integration-cluster",
  "namespace":"{{ .Event.Namespace }}",
  "pod":"{{ .Event.Name }}",
  "ruleName":"{{ .RuleName }}",
  "source":"kube-watcher"
}`,
					},
				},
				Settings: config.Settings{QueueDepth: 8},
			}

			store := tracker.NewMemoryStore()
			t.Cleanup(func() { _ = store.Close() })

			engine := rules.NewEngine(logger, cfg, store, nil)
			dispatcher, err := actions.NewDispatcher(logger, cfg.Actions, 4)
			require.NoError(t, err)
			t.Cleanup(dispatcher.Close)

			src := eventSource{
				events: []events.ResourceEvent{
					{
						Kind:      "Pod",
						Namespace: "payments",
						Name:      "checkout-api-7ff98f4bcf-6jwkq",
						Object: map[string]any{
							"metadata":       map[string]any{"name": "checkout-api-7ff98f4bcf-6jwkq"},
							"restart_delta":  1,
							"waiting_reason": tc.waitingReason,
						},
					},
				},
			}

			pipe := pipeline.New(
				logger,
				src,
				pipeline.NewRuleAwareFilter(cfg),
				nil,
				engine,
				dispatcher,
				store,
				nil,
				cfg.Settings.QueueDepth,
				2,
				10,
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go pipe.Start(ctx)

			select {
			case signal := <-signalCh:
				assert.Equal(t, tc.alertKind, signal.AlertPayload["kind"])
				assert.Equal(t, "payments", signal.AlertPayload["namespace"])
				assert.Equal(t, "checkout-api-7ff98f4bcf-6jwkq", signal.AlertPayload["pod"])
				assert.Equal(t, "pod_failure_to_ui", signal.AlertPayload["ruleName"])

				assert.Equal(t, "payments", signal.MCPPayload["namespace"])
				assert.Equal(t, "checkout-api-7ff98f4bcf-6jwkq", signal.MCPPayload["pod"])
				assert.Equal(t, tc.alertKind, signal.MCPPayload["kind"])
				assert.Equal(t, float64(50), signal.MCPPayload["lines"])
			case <-time.After(3 * time.Second):
				t.Fatalf("timed out waiting for engine->ui->mcp chain. logs: %s", logBuffer.String())
			}
		})
	}
}
