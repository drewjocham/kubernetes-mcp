# NexusTerm Implementation Checklist

## Completed

- [x] `kw terminal` command wired in CLI
- [x] PTY adapter and core terminal handler
- [x] Bubble Tea AltScreen + mouse support
- [x] Block manager and block-based rendering flow
- [x] Render strategy abstraction (plain + markdown)
- [x] Ghost text message handling in input UI
- [x] Artifact pin/tab side panel rendering
- [x] Unix socket bridge with basic Agent actions
- [x] Diagnostic event emission on non-zero PTY exit

## In Progress / Next

### Agent Socket & Orchestration

- [x] Add split/pane actions:
  - [x] `create_split`
  - [x] `focus_split`
  - [x] `send_input`
  - [x] `open_url`
- [x] Add command validation + safe defaults
- [x] Add socket permission hardening (`0600`)
- [x] Add richer event stream (`ghost_applied`, `artifact_pinned`, `command_executed`)

### Browser Integration (Playwright)

- [x] Add browser adapter interface in domain layer
- [x] Implement Playwright adapter lifecycle (`start`, `navigate`, `snapshot`, `close`)
- [x] Add `open_url` socket action -> browser snapshot -> artifact pin
- [x] Normalize snapshot output into terminal-friendly markdown/text

### Block UX Improvements

- [x] Attach command lifecycle metadata per block:
  - [x] cwd
  - [x] duration
  - [x] status badges
- [x] Add max-block retention policy + pruning
- [x] Add explicit markdown mode toggle per block
- [x] Add copy-output action on selected block

### Desktop-like Input UX

- [x] Add `Tab` accept for ghost text
- [x] Add shortcut-driven artifact tab switching
- [x] Improve mouse targeting for block and input hit-testing
- [x] Evaluate `textarea` for multiline editor mode (deferred for MVP)

### Widgets

- [x] Define widget interface (`ID`, `Title`, `Refresh`, `View`)
- [x] Implement System Stats widget
- [x] Implement Git Status widget
- [x] Add widget panel mount + refresh scheduling

### Reliability & Tests

- [ ] Table-driven unit tests:
  - [x] block manager behavior
  - [x] markdown detection strategy
  - [x] socket request routing and validation
  - [x] ghost/artifact/diagnostic message handling
- [x] Integration test for `kw terminal` startup + socket actions
- [x] Add graceful shutdown tests for PTY/socket/browser resources

## Suggested order of execution

1. Socket split/open_url action schema
2. Playwright adapter + artifact pin flow
3. Widget system
4. Input/ghost UX polish
5. Full test pass and hardening
