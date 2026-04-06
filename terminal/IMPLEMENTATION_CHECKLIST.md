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

- [ ] Attach command lifecycle metadata per block:
  - [ ] cwd
  - [ ] duration
  - [ ] status badges
- [ ] Add max-block retention policy + pruning
- [ ] Add explicit markdown mode toggle per block
- [ ] Add copy-output action on selected block

### Desktop-like Input UX

- [ ] Add `Tab` accept for ghost text
- [ ] Add shortcut-driven artifact tab switching
- [ ] Improve mouse targeting for block and input hit-testing
- [ ] Evaluate `textarea` for multiline editor mode

### Widgets

- [ ] Define widget interface (`ID`, `Title`, `Refresh`, `View`)
- [ ] Implement System Stats widget
- [ ] Implement Git Status widget
- [ ] Add widget panel mount + refresh scheduling

### Reliability & Tests

- [ ] Table-driven unit tests:
  - [ ] block manager behavior
  - [ ] markdown detection strategy
  - [x] socket request routing and validation
  - [ ] ghost/artifact/diagnostic message handling
- [ ] Integration test for `kw terminal` startup + socket actions
- [ ] Add graceful shutdown tests for PTY/socket/browser resources

## Suggested order of execution

1. Socket split/open_url action schema
2. Playwright adapter + artifact pin flow
3. Widget system
4. Input/ghost UX polish
5. Full test pass and hardening
