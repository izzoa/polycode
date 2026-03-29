# UX/UI Polish & Feature Plan for Polybot TUI

This document outlines high-impact UI/UX features inspired by modern coding assistants (like Crush, Opencode, and Cursor) that can be adapted for Polybot's Bubble Tea-based Terminal User Interface. 

The goal is to increase transparency, build trust during autonomous tool loops, and reduce friction for the user.

## 1. Context Input & Discovery

### Inline Context Referencing (`@mentions`)
* **What it is:** The ability to type `@` in the chat input to search and quickly pull files, folders, web URLs, or LSP symbols into the prompt's context.
* **Why it matters:** Users currently have to type out full paths (`internal/tui/model.go`) or rely on the agent to finding files. Direct inclusion gives the user fine-grained control over exactly what the agent should read.
* **Implementation Strategy:** Combine Charm's `textinput` or `textarea` with a floating list (`bubbles/list` or `charmbracelet/bubbles/table`). Triggering `@` spawns the list populated via `glob`, `find`, or an LSP integration. Selecting an item injects absolute paths or content snippets seamlessly.

### Session Persistence & Command History
* **What it is:** Pressing `↑` in the chat input restores previous queries, and past threads/conversations can be resumed cleanly.
* **Why it matters:** Coding often involves iterative refinement. Starting over or re-typing complex prompts is painful.
* **Implementation Strategy:** Maintain a local SQLite or JSON append-only log of user prompts history list. Wire the `up/down` arrows in the text area to cycle this history when empty or at boundaries.

## 2. Autonomous Agent Observability

### Live Task / Tool HUD (Todo Tracking)
* **What it is:** A dynamic checklist or sticky footer displaying the model's internal plan and current action (e.g., `[✓] Read files`, `[➤] Modifying server.go`, `[ ] Running tests`).
* **Why it matters:** Staring at a generic "Thinking..." spinner during a 30-second multi-step tool loops causes anxiety. The user needs to know the bot isn't stuck.
* **Implementation Strategy:** Use `bubbles/spinner` and a compacted list view. When the agent uses a tool, emit a Bubble Tea `Msg` to append/update the HUD list block in the UI.

### Real-time Log Streaming for `shell_exec`
* **What it is:** A transient, scrollable viewport showing live `stdout`/`stderr` when the agent runs long-running bash commands (like `npm install` or test suites).
* **Why it matters:** Ensures the user instantly knows if a build is failing or hanging on a user prompt.
* **Implementation Strategy:** Tie the `shell_exec` tool's executor to an `io.Pipe` or `pty` that streams directly into a Charm `viewport` or `list` bubble updated via `tea.Tick`.

### Fan-out Consensus Visibility
* **What it is:** Because Polycode queries multiple LLMs in parallel, a dedicated visual component should show the state of each provider.
* **Why it matters:** Exposes the unique value proposition of Polycode. Users can see if "Gemini is slow but Claude is done," or how the consensus is forming.
* **Implementation: ** A horizontal status bar or animated grid showing cards for each provider: `Anthropic: Done`, `OpenAI: Typing...`, `Primary: Synthesizing`.

## 3. Safe & Actionable Tool Execution

### Pre-flight Actionable Diffs
* **What it is:** When the agent executes `file_edit` or `multiedit`, the TUI renders a compact, syntax-highlighted unified diff before or immediately after execution.
* **Why it matters:** Fosters immense trust. Users shouldn't have to exit the app or run `git diff` separately to verify the bot didn't delete half a file.
* **Implementation Strategy:** Use `go-diff` or a similar library to generate patch lines, format them with red/green (`lipgloss`), and render them in the chat feed as a discrete bubble block.

### Editable "Approve" Gates
* **What it is:** Instead of a strict `(y/n)` for mutating tools (like `shell_exec` or `file_write`), offer an "Edit" keybind (`e`) that lets the user native-edit the command or proposed text block before it actually fires.
* **Why it matters:** If the bot writes an amazing 50-line command but makes one flag mistake, the user currently has to reject it and re-prompt. Editing saves time.
* **Implementation Strategy:** When waiting for confirmation, if `e` is pressed, swap the view to a `textarea` containing the tool's JSON payload or command string. On submit, pass the modified version to the executor.

## 4. UI Polish & Typography

### Streaming Markdown Parsing
* **What it is:** Code blocks should syntax highlight *as the model types them*, rather than waiting for the whole block to finish.
* **Why it matters:** Makes the UI feel incredibly fast and polished.
* **Implementation Strategy:** Enhance the `glamour` markdown renderer or use a custom scanner that applies `lipgloss` code block styling heuristically on incomplete fences during the streaming `ConsensusChunkMsg` phase.

### Error Recovery UI (Auto-fix Prompts)
* **What it is:** When a tool fails (e.g., tests fail or `find` errors out), present actionable buttons or hotkeys (e.g., `[R]etry`, `[A]uto-fix: send error to model`).
* **Why it matters:** Turns dead-ends into natural conversation loops.
* **Implementation Strategy:** Catch tool executor errors and append an interactive "Error Block" to the chat feed rather than just raw text. Allow a keypress to automatically generate a `user` prompt wrapping the error trace.
