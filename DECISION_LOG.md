# Decision Log

| ID | Decision | Impact | Proposal | Date | Status |
|----|----------|--------|----------|------|--------|
| SEC-01 | Add PII sanitizer to watcher log streaming | High | Implement a security sanitizer that filters sensitive fields (environment variables, secret references, configmap references) and uses regex patterns for common secret patterns (API_KEY, SECRET, PASSWORD, TOKEN, etc.) in string values. Use a deny-list approach that can be extended via configuration. | 2025-04-07 | Implemented |
| CONC-01 | Improve error handling in MCP server HTTP server goroutine | Medium | On HTTP server startup failure (e.g., port in use), log the error and continue without HTTP API (degraded mode). If the HTTP server fails after starting (runtime error), attempt one restart, then continue degraded. | 2025-04-07 | Implemented |
| DOC-01 | Update AGENTS.md with accurate commands and fix references | Low | Update test commands to reflect actual Make targets (`make test-integration-channel`, `make run-watcher`, `make run-server`). Fix DRY rule reference from `pkg/common` to existing packages in `pkg/` and `internal/`. | 2025-04-07 | Implemented |

## Notes

- SEC-01: Sanitizer implemented in `watcher/internal/security/sanitizer.go`. Integrated into watcher's `Observe` method. Redacts sensitive fields and values, clears Raw field to prevent leakage.
- CONC-01: Modified `mcp/cmd/server/main.go` HTTP server goroutine with retry logic (max 2 attempts). Logs degraded mode after failures.
- DOC-01: AGENTS.md updated to reflect current project state.

