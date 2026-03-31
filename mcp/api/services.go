package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
)

// ServiceStatus is the wire type for a Docker container's status.
type ServiceStatus struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	Status    string `json:"status"` // running | exited | not_found
	StartedAt string `json:"startedAt,omitempty"`
	ID        string `json:"id,omitempty"`
}

// knownServices lists the compose service container name prefixes the TUI cares about.
var knownServices = []string{
	"kw-mcp",
	"kw-watcher",
	"prometheus",
	"grafana",
}

func (a *API) handleListServices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cli, err := newDockerAPIClient()
	if err != nil {
		// Docker not available — return all unknown
		statuses := make([]ServiceStatus, len(knownServices))
		for i, n := range knownServices {
			statuses[i] = ServiceStatus{Name: n, Status: "unknown"}
		}
		a.respond(w, r, http.StatusOK, map[string]any{"services": statuses})
		return
	}
	defer func() { _ = cli.Close() }()

	statuses := make([]ServiceStatus, 0, len(knownServices))
	for _, name := range knownServices {
		info, err := cli.ContainerInspect(ctx, name)
		if err != nil {
			statuses = append(statuses, ServiceStatus{Name: name, Status: "not_found"})
			continue
		}
		status := "unknown"
		startedAt := ""
		if info.State != nil {
			status = info.State.Status
			startedAt = info.State.StartedAt
		}
		image := ""
		if info.Config != nil {
			image = info.Config.Image
		}
		statuses = append(statuses, ServiceStatus{
			Name:      name,
			Image:     image,
			Status:    status,
			StartedAt: startedAt,
			ID:        info.ID[:min(12, len(info.ID))],
		})
	}
	a.respond(w, r, http.StatusOK, map[string]any{"services": statuses})
}

func (a *API) handleStartService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !isKnownService(name) {
		a.respondError(w, r, http.StatusBadRequest, "unknown service", fmt.Errorf("service %q not in managed list", name))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	cli, err := newDockerAPIClient()
	if err != nil {
		a.respondError(w, r, http.StatusServiceUnavailable, "docker unavailable", err)
		return
	}
	defer func() { _ = cli.Close() }()

	if err := cli.ContainerStart(ctx, name, container.StartOptions{}); err != nil {
		a.respondError(w, r, http.StatusInternalServerError, "start failed", err)
		return
	}
	a.respond(w, r, http.StatusOK, map[string]string{"status": "started", "name": name})
}

func (a *API) handleStopService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !isKnownService(name) {
		a.respondError(w, r, http.StatusBadRequest, "unknown service", fmt.Errorf("service %q not in managed list", name))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	cli, err := newDockerAPIClient()
	if err != nil {
		a.respondError(w, r, http.StatusServiceUnavailable, "docker unavailable", err)
		return
	}
	defer func() { _ = cli.Close() }()

	timeout := 10
	if err := cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout}); err != nil {
		a.respondError(w, r, http.StatusInternalServerError, "stop failed", err)
		return
	}
	a.respond(w, r, http.StatusOK, map[string]string{"status": "stopped", "name": name})
}

func (a *API) handleServiceLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}

	ctx := r.Context()

	cli, err := newDockerAPIClient()
	if err != nil {
		http.Error(w, "docker unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer func() { _ = cli.Close() }()

	follow := r.URL.Query().Get("follow") == "true"

	reader, err := cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
		Timestamps: true,
	})
	if err != nil {
		http.Error(w, "logs error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = reader.Close() }()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// Docker log stream has an 8-byte header per line; strip it
		if len(line) > 8 {
			line = line[8:]
		}
		_, _ = fmt.Fprintln(w, strings.TrimRight(line, "\r\n"))
		if canFlush {
			flusher.Flush()
		}
	}
}

func (a *API) handleRestartService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !isKnownService(name) {
		a.respondError(w, r, http.StatusBadRequest, "unknown service", fmt.Errorf("service %q not in managed list", name))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	cli, err := newDockerAPIClient()
	if err != nil {
		a.respondError(w, r, http.StatusServiceUnavailable, "docker unavailable", err)
		return
	}
	defer func() { _ = cli.Close() }()

	timeout := 10
	if err := cli.ContainerRestart(ctx, name, container.StopOptions{Timeout: &timeout}); err != nil {
		a.respondError(w, r, http.StatusInternalServerError, "restart failed", err)
		return
	}
	a.respond(w, r, http.StatusOK, map[string]string{"status": "restarted", "name": name})
}

func newDockerAPIClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func isKnownService(name string) bool {
	for _, s := range knownServices {
		if s == name {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// drain helper so the compiler doesn't complain about the unused io.Writer
var _ io.Writer
