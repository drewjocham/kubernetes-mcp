# ─── Terminal Module ──────────────────────────────────────────────────────────
# Covers: launching terminal TUI and terminal-only tests

.PHONY: terminal run-terminal test-terminal

terminal: run-terminal

run-terminal: build-cli
	@echo "Launching terminal module via kw terminal..."
	@$(BIN_DIR)/kw terminal

test-terminal:
	@echo "Running terminal module tests..."
	$(GOTEST) -v ./terminal/...
