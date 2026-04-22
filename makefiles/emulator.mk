# ─── Pub/Sub Emulator for Local Testing ──────────────────────────────────────

.PHONY: emulator-up emulator-down emulator-logs emulator-publish-test \
	emulator-test-integration emulator-health

# Docker Compose file for emulator
EMULATOR_COMPOSE_FILE ?= $(ROOT_DIR)/docker-compose.emulator.yaml

# ── Docker Compose Operations ─────────────────────────────────────────────────

emulator-up:
	@echo "Starting Pub/Sub emulator..."
	docker compose -f $(EMULATOR_COMPOSE_FILE) up -d --wait

emulator-down:
	@echo "Stopping Pub/Sub emulator..."
	docker compose -f $(EMULATOR_COMPOSE_FILE) down --volumes

emulator-logs:
	@echo "Tailing Pub/Sub emulator logs..."
	docker compose -f $(EMULATOR_COMPOSE_FILE) logs -f

emulator-health:
	@echo "Checking emulator health..."
	@if curl -s http://localhost:8085 >/dev/null 2>&1; then \
		echo "✅ Pub/Sub emulator is healthy"; \
	else \
		echo "❌ Pub/Sub emulator is not responding"; \
		exit 1; \
	fi

# ── Test Operations ───────────────────────────────────────────────────────────

emulator-publish-test: emulator-health
	@echo "Publishing test anomalies to emulator..."
	@cd $(ROOT_DIR)/test && \
		PUBSUB_EMULATOR_HOST=localhost:8085 \
		GOOGLE_CLOUD_PROJECT=test-project \
		python publish_test_anomalies.py \
			--project test-project \
			--topic anomaly-events-topic \
			--count 3 \
			--emulator localhost:8085

# Run watcher with emulator configuration
run-watcher-emulator: build-watcher
	@echo "Running watcher with Pub/Sub emulator..."
	@PUBSUB_EMULATOR_HOST=localhost:8085 \
	GOOGLE_CLOUD_PROJECT=test-project \
	$(GORUN) $(WATCHER_CMD) \
		--config $(ROOT_DIR)/watcher/internal/config/config.yaml

# Integration test with emulator
emulator-test-integration: emulator-up
	@echo "Running integration test with Pub/Sub emulator..."
	@PUBSUB_EMULATOR_HOST=localhost:8085 \
	GOOGLE_CLOUD_PROJECT=test-project \
	$(GOTEST) -v ./watcher/internal/source -run "TestPubSub"
	@$(MAKE) emulator-down

# ── Environment Setup ────────────────────────────────────────────────────────

# Export environment variables for emulator
export-emulator-env:
	@echo "Exporting Pub/Sub emulator environment variables..."
	@echo "export PUBSUB_EMULATOR_HOST=localhost:8085"
	@echo "export GOOGLE_CLOUD_PROJECT=test-project"
	@echo "unset GOOGLE_APPLICATION_CREDENTIALS"