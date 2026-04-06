# NexusTerm (kube-watcher/terminal)

AI-native, block-based terminal built with Go + Bubble Tea.

## Current MVP capabilities

- PTY-backed shell session (`creack/pty`)
- Bubble Tea AltScreen terminal UI with mouse support
- Block-based output model (not a single scroll buffer)
- Click a previous block to reuse its command in the active input
- Markdown/plain rendering strategies (Glamour + plain fallback)
- Ghost text support in the input via Agent messages
- Artifact side panel with pinnable text/browser snapshot content
- Unix socket bridge for Agent control and context access
- Diagnostic event emission on non-zero PTY exit

## CLI

Run from repository root:

```bash
kw terminal
```

## Agent socket API (current)

Socket server is started by the terminal app and supports:

- `get_blocks`
- `get_events`
- `set_ghost_text`
- `clear_ghost_text`
- `pin_artifact`
- `create_split`
- `focus_split`
- `send_input`
- `open_url`

Request fields:
- `action` (required)
- `text` (for `set_ghost_text`, `send_input`)
- `artifact` (for `pin_artifact`)
- `split_id` (for `create_split`, `focus_split`; auto-generated for `create_split` when omitted)
- `url` (for `open_url`, must be `http` or `https`)

`open_url` behavior:
- Executes browser adapter flow (`start` → `navigate` → `snapshot`)
- Normalizes snapshot to markdown and pins it as an artifact
- Falls back to UI-level open-url message if no browser handler is wired

Example payload:

```json
{
  "action": "set_ghost_text",
  "text": "go test ./..."
}
```

## Project structure

```text
terminal/
├── app/
│   └── run.go
├── internal/
│   ├── adapters/
│   │   ├── markdown/
│   │   └── pty/
│   ├── agent/
│   │   └── socket/
│   ├── core/
│   │   ├── blocks/
│   │   └── terminal/
│   ├── domain/
│   └── ui/
│       └── tui/
└── readme.md
```

## Development notes

- UI state is updated through Bubble Tea messages only.
- PTY/browser/socket logic is adapter-based and kept outside core UI state.
- Current input is modeless and ships command text to PTY on `Enter`.

## Next major additions

- Socket actions for split control (`create_split`, `focus_split`, `send_input`)
- Browser `open_url` command flow via Playwright adapter
- Widget framework (System Stats + Git Status)
- Table-driven tests for block manager, socket protocol, and render strategies
