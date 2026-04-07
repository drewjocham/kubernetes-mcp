# NexusTerm (kube-watcher/terminal)

AI-native, block-based terminal built with Go + Bubble Tea.

🏗️  Architecture for the Agent
Persistence & Memory Safety:

Memory Ceiling: Implement a MaxLiveBlocks (e.g., 50). When reached, the oldest block is evicted from RAM and sent to the Persistence Adapter.

Background Persistence: Use a buffered channel to send blocks to BadgerDB. The TUI Update loop must never call db.Update directly.

On-Demand Hydration: If a user scrolls to an evicted block, the BlockManager fetches it from the DB asynchronously and emits a tea.Msg to refresh the UI.


This is a solid, professional-grade specification. It accurately captures the **OpenCode** vision while maintaining the strict architectural boundaries required for a performant Go application.

To round out the **BadgerDB Schema** and the **Background Indexing** logic you'll need for the AI to be truly "context-aware," add this final technical section to your document. This ensures the Agent can query the database for "Semantic History" without hitting the PTY.

---

## 🗄️ Persistence Schema (BadgerDB)

To allow the Agent to search history efficiently, we use a **Key-Prefix** strategy. This allows for range scans (e.g., "all blocks from today") and specific lookups.

| Key Pattern | Value (JSON) | Purpose |
| :--- | :--- | :--- |
| `block:{uuid}` | `domain.Block` | The raw content and metadata. |
| `idx:time:{timestamp}:{uuid}` | `{uuid}` | Chronological index for scrolling. |
| `idx:cwd:{hash(path)}:{uuid}` | `{uuid}` | Filter history by directory. |
| `idx:exit:{code}:{uuid}` | `{uuid}` | Quickly find all failed commands. |



### The "Hydration" Flow
1. **User Scrolls Up:** The TUI detects the scroll offset has hit the top of the `MaxLiveBlocks`.
2. **Async Fetch:** A `tea.Cmd` triggers `badger.View`. It scans `idx:time` backwards from the oldest live block.
3. **Insert:** The `BlockManager` receives the `[]domain.Block` and prepends them to the view.

---

## 🛠️ Final Implementation Checklist for the Agent

Add these specific "User Experience" guardrails to your **Next Major Additions**:

### 1. The "Ghost Buffer" ANSI Sanitizer
* **Task:** Implement a middleware in `internal/core/blocks` that interprets backspaces (`\b`) and carriage returns (`\r`) before storing them in Badger.
* **Why:** If you save raw PTY noise to the DB, the AI will see `ls\b\bcat` instead of `cat`.

### 2. Smart PTY Throttling
* **Task:** Use a `time.Ticker` (e.g., 16ms for 60FPS) to batch PTY reads into a single `tea.Msg`.
* **Why:** High-velocity output (like `yarn install`) will flood the Bubble Tea event loop with thousands of messages per second, making the UI unresponsive.

### 3. Desktop Clipboard Integration
* **Task:** Add a `CopyBlock` action.
* **UI:** When a block is focused, `Cmd+C` should copy the `Block.RawOutput` to the system clipboard using `atotto/clipboard`.

---

## 🚀 for the the Agent

Your `README.md` and `SPEC` are now complete. When you provide this to the agent, use the following **Final Kickoff Message**:

> "I have provided the full Architecture, Design Patterns, and Socket API.
>
> **Your specific goal for the next 2 hours:**
> 1. Scaffold the `terminal/` directory.
> 2. Implement the `PTYAdapter` and the `BlockManager`.
> 3. Create a `tea.Model` that renders shell output in bordered blocks.
> 4. Implement the **BadgerDB persistence layer** with the 'Hot/Cold' memory ceiling (50 blocks).


## Current MVP capabilities

- PTY-backed shell session (`creack/pty`)
- Bubble Tea AltScreen terminal UI with mouse support
- Block-based output model (not a single scroll buffer)
- Click a previous block to reuse its command in the active input
- Markdown/plain rendering strategies (Glamour + plain fallback)
- Ghost text support in the input via Agent messages
- Built-in slash command system (`/help`, `/agent`, `/widgets`)
- Artifact side panel with pinnable text/browser snapshot content
- Widget sidebar support (System Stats + Git Status, periodic refresh)
- Unix socket bridge for Agent control and context access
- Diagnostic event emission on non-zero PTY exit

## CLI

Run from repository root:

```bash
kw terminal
# or
make run-terminal
# run terminal module tests
make test-terminal
```

## Built-in slash commands

- `/help` — list available terminal system commands
- `/agent` — show agent/socket capabilities
- `/widgets` — list registered widgets
- `/widgets refresh` — refresh widgets immediately

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
- Input UX polish:
  - `Tab` accepts ghost text when suggestion extends current input
  - `Tab` / `Shift+Tab` cycle artifact tabs when no ghost acceptance is available
  - Mouse hit-testing prioritizes sidebar tab selection, then input, then block selection
- Block UX improvements:
  - Block header badges include status, render mode, cwd basename, and command duration
  - Live block retention cap is enforced at 50 blocks with oldest-first pruning
  - `Ctrl+T` toggles selected block render mode (`auto` → `markdown` → `plain`)
  - `Ctrl+Shift+C` copies selected block output to system clipboard
- Multiline editor mode evaluation:
  - `textarea` migration is evaluated and deferred for now to keep command execution semantics stable in the MVP.

## Next major additions

- Socket actions for split control (`create_split`, `focus_split`, `send_input`)
- Browser `open_url` command flow via Playwright adapter
- Widget framework (System Stats + Git Status)
- Table-driven tests for block manager, socket protocol, and render strategies
