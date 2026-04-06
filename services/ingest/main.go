package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub" //nolint:staticcheck // SA1019: v1 client; migrate to pubsub/v2 when subscription admin is refactored.
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/errgroup"

	"kube-watcher/pkg/agenttelemetry"
)

var (
	projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	topicID   = "agent-telemetry-topic"
	client    *pubsub.Client
	topic     *pubsub.Topic
)

func initPubSub(ctx context.Context) error {
	if projectID == "" {
		log.Println("GOOGLE_CLOUD_PROJECT not set, running in mock mode (no Pub/Sub)")
		client = nil
		topic = nil
		return nil
	}
	var err error
	client, err = pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("failed to create pubsub client: %w", err)
	}
	topic = client.Topic(topicID)
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := initPubSub(ctx); err != nil {
		log.Fatalf("Initialization error: %v", err)
	}
	if client != nil {
		defer func() {
			if cerr := client.Close(); cerr != nil {
				log.Printf("pubsub client close: %v", cerr)
			}
		}()
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/ingest", handleIngest)
	r.Get("/healthz", handleHealth)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Println("Starting HTTP server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s := <-sig:
			log.Printf("Received signal %v, shutting down", s)
			cancel()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("server shutdown error: %w", err)
			}
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handleIngest(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Agent-Token")
	if token != os.Getenv("AGENT_SECRET_TOKEN") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var envelope map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if m, ok := envelope["metrics"]; ok {
		envelope["metrics"] = agenttelemetry.NormalizeMetrics(m)
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	agentID, _ := envelope["agent_id"].(string)
	log.Printf("Received telemetry from agent %s", agentID)
	if topic != nil {
		msg := &pubsub.Message{
			Data: data,
			Attributes: map[string]string{
				"agent_id": agentID,
			},
		}

		result := topic.Publish(r.Context(), msg)

		go func(res *pubsub.PublishResult) {
			_, err := res.Get(context.Background())
			if err != nil {
				log.Printf("Failed to publish message: %v", err)
			}
		}(result)
	}

	w.WriteHeader(http.StatusAccepted)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		log.Printf("health encode: %v", err)
	}
}
