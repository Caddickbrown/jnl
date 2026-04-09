# Built-in Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Embed a fully functional terminal text editor into jnl so it works on Windows, macOS, and Linux without any external editor installed.

**Architecture:** A new `editor.go` file contains a bubbletea model (`editorModel`) with a textarea component, a 100-entry undo stack, and save/quit handling. `openInEditor` in `main.go` gains fallback logic: use `$EDITOR` if set, try `micro` if on PATH, otherwise call `runBuiltinEditor`. The global `editor` variable is removed.

**Tech Stack:** `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles/textarea`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `editor.go` | Create | `editorModel`, `runBuiltinEditor`, `errQuitWithoutSaving` |
| `editor_test.go` | Create | Unit tests for the model and undo stack |
| `main.go` | Modify | Replace `openInEditor`, remove `editor` global, handle `errQuitWithoutSaving` in `cmdNew` |
| `go.mod` / `go.sum` | Modify | Add bubbletea and bubbles |

---

## Task 1: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add bubbletea and bubbles**

```bash
cd /home/dcb/jnlport
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
```

Expected output: lines like `go: added github.com/charmbracelet/bubbletea v0.x.x`

- [ ] **Step 2: Verify the build still passes**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add bubbletea and bubbles for built-in editor"
```

---

## Task 2: editorModel — TDD

**Files:**
- Create: `editor_test.go`
- Create: `editor.go`

- [ ] **Step 1: Write failing tests**

Create `/home/dcb/jnlport/editor_test.go`:

```go
package main

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewEditorModel_SetsContent(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "hello world")
	if got := m.textarea.Value(); got != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got)
	}
	if m.status != editorEditing {
		t.Errorf("expected editorEditing, got %v", m.status)
	}
	if len(m.undoStack) != 0 {
		t.Errorf("expected empty undo stack on init, got %d entries", len(m.undoStack))
	}
}

func TestEditorModel_TypingPushesUndoStack(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(editorModel)

	if len(m.undoStack) != 1 {
		t.Fatalf("expected 1 undo entry after typing, got %d", len(m.undoStack))
	}
	if m.undoStack[0] != "" {
		t.Errorf("expected empty string snapshot, got %q", m.undoStack[0])
	}
	if m.textarea.Value() != "a" {
		t.Errorf("expected value %q, got %q", "a", m.textarea.Value())
	}
}

func TestEditorModel_CtrlZ_RestoresPreviousSnapshot(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "")

	// Type two characters so we have two snapshots
	for _, r := range "ab" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(editorModel)
	}
	// undoStack: ["", "a"], value: "ab"

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = result.(editorModel)

	if m.textarea.Value() != "a" {
		t.Errorf("expected %q after undo, got %q", "a", m.textarea.Value())
	}
	if len(m.undoStack) != 1 {
		t.Errorf("expected 1 undo entry remaining, got %d", len(m.undoStack))
	}
}

func TestEditorModel_CtrlZ_NoopOnEmptyStack(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "hello")

	// No typing — stack is empty
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	m = result.(editorModel)

	if m.textarea.Value() != "hello" {
		t.Errorf("expected value unchanged, got %q", m.textarea.Value())
	}
	if m.status != editorEditing {
		t.Errorf("expected editorEditing status")
	}
}

func TestEditorModel_CtrlS_SetsSavedStatus(t *testing.T) {
	tmp, err := os.CreateTemp("", "jnl_test_*.md")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	m := newEditorModel(tmp.Name(), "entry text")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	fm := result.(editorModel)

	if fm.status != editorSaved {
		t.Errorf("expected editorSaved, got %v", fm.status)
	}

	written, _ := os.ReadFile(tmp.Name())
	if string(written) != "entry text" {
		t.Errorf("expected file to contain %q, got %q", "entry text", string(written))
	}
}

func TestEditorModel_CtrlQ_SetsQuitStatus(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "hello")
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	fm := result.(editorModel)

	if fm.status != editorQuit {
		t.Errorf("expected editorQuit, got %v", fm.status)
	}
}

func TestEditorModel_EscEsc_SetsQuitStatus(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "hello")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(editorModel)
	if m.status == editorQuit {
		t.Error("single Esc should not quit")
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(editorModel)
	if m.status != editorQuit {
		t.Errorf("double Esc should quit, got status %v", m.status)
	}
}

func TestEditorModel_UndoStack_CappedAt100(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "")

	// Manually stuff the undo stack to 100
	for i := 0; i < 100; i++ {
		m.undoStack = append(m.undoStack, "x")
	}

	// Type one more character — should evict the oldest entry
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(editorModel)

	if len(m.undoStack) != 100 {
		t.Errorf("expected undo stack capped at 100, got %d", len(m.undoStack))
	}
}

func TestEditorModel_TypingResetsEscCount(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "")

	// Press Esc once
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(editorModel)
	if m.escCount != 1 {
		t.Fatalf("expected escCount 1, got %d", m.escCount)
	}

	// Type a character — should reset escCount
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(editorModel)
	if m.escCount != 0 {
		t.Errorf("expected escCount reset to 0 after typing, got %d", m.escCount)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /home/dcb/jnlport && go test ./... -run TestEditorModel -v 2>&1 | head -20
```

Expected: compile error — `newEditorModel`, `editorModel`, `editorEditing` etc. undefined.

- [ ] **Step 3: Create editor.go**

Create `/home/dcb/jnlport/editor.go`:

```go
package main

import (
	"errors"
	"os"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

var errQuitWithoutSaving = errors.New("quit without saving")

const maxUndoStack = 100

type editorStatus int

const (
	editorEditing editorStatus = iota
	editorSaved
	editorQuit
)

type editorModel struct {
	textarea  textarea.Model
	undoStack []string
	path      string
	status    editorStatus
	escCount  int
}

func newEditorModel(path, content string) editorModel {
	ta := textarea.New()
	ta.SetValue(content)
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetWidth(80)
	ta.SetHeight(20)
	return editorModel{
		textarea: ta,
		path:     path,
		status:   editorEditing,
	}
}

func (m editorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m editorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(msg.Height - 2)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			_ = os.WriteFile(m.path, []byte(m.textarea.Value()), 0644)
			m.status = editorSaved
			return m, tea.Quit

		case "ctrl+q":
			m.status = editorQuit
			return m, tea.Quit

		case "esc":
			m.escCount++
			if m.escCount >= 2 {
				m.status = editorQuit
				return m, tea.Quit
			}
			return m, nil

		case "ctrl+z":
			if len(m.undoStack) > 0 {
				prev := m.undoStack[len(m.undoStack)-1]
				m.undoStack = m.undoStack[:len(m.undoStack)-1]
				m.textarea.SetValue(prev)
			}
			return m, nil
		}

		prev := m.textarea.Value()
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		if m.textarea.Value() != prev {
			m.undoStack = append(m.undoStack, prev)
			if len(m.undoStack) > maxUndoStack {
				m.undoStack = m.undoStack[1:]
			}
		}
		m.escCount = 0
		return m, cmd
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m editorModel) View() string {
	return m.textarea.View() + "\n\nCtrl+S save · Ctrl+Z undo · Ctrl+Q quit"
}

func runBuiltinEditor(path string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	m := newEditorModel(path, string(content))
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	if final.(editorModel).status == editorQuit {
		return errQuitWithoutSaving
	}
	return nil
}
```

- [ ] **Step 4: Run tests and verify they pass**

```bash
cd /home/dcb/jnlport && go test ./... -run TestEditorModel -v
```

Expected: all 8 tests PASS.

- [ ] **Step 5: Verify the full build**

```bash
go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 6: Commit**

```bash
git add editor.go editor_test.go
git commit -m "feat: add built-in editor using bubbletea"
```

---

## Task 3: Wire built-in editor into openInEditor

**Files:**
- Modify: `main.go` lines 34–48 (remove `editor` var/init), lines 261–271 (`openInEditor`), lines 348–351 (`cmdNew` error handling)

- [ ] **Step 1: Write a failing test for the quit-without-saving path in cmdNew**

Add to `editor_test.go`:

```go
func TestRunBuiltinEditor_QuitWithoutSaving_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires TTY")
	}
	// This test documents the contract: errQuitWithoutSaving is the sentinel.
	// Full integration test is manual (requires a real terminal).
	if errQuitWithoutSaving.Error() != "quit without saving" {
		t.Errorf("unexpected sentinel error text: %q", errQuitWithoutSaving.Error())
	}
}
```

- [ ] **Step 2: Run to confirm it passes (contract test)**

```bash
cd /home/dcb/jnlport && go test ./... -run TestRunBuiltinEditor -v
```

Expected: PASS.

- [ ] **Step 3: Remove the `editor` global variable from main.go**

In `main.go`, remove line 38 (`editor string`) from the var block and line 46 (`editor = envOr("EDITOR", "micro")`) from `init()`.

The var block at lines 34–39 becomes:

```go
var (
	notesDir   string
	inboxPath  string
	journalDir string
)
```

The `init()` function at lines 41–48 becomes:

```go
func init() {
	home, _ := os.UserHomeDir()
	notesDir = envOr("JNL_DIR", filepath.Join(home, "notes"))
	inboxPath = filepath.Join(notesDir, "inbox.md")
	journalDir = filepath.Join(notesDir, "journal")
	os.MkdirAll(journalDir, 0755)
}
```

- [ ] **Step 4: Replace openInEditor in main.go**

Replace lines 259–271 (the entire `openInEditor` function):

```go
// ── editor ────────────────────────────────────────────────────────────────────

func openInEditor(path string) error {
	// User explicitly set EDITOR — use it unconditionally.
	if editorEnv := os.Getenv("EDITOR"); editorEnv != "" {
		args := []string{path}
		if strings.Contains(editorEnv, "micro") {
			args = append([]string{"-filetype", "jnl-markdown", "+99999"}, args...)
		}
		cmd := exec.Command(editorEnv, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// EDITOR not set — try micro if available, otherwise use built-in.
	if microPath, err := exec.LookPath("micro"); err == nil {
		cmd := exec.Command(microPath, "-filetype", "jnl-markdown", "+99999", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return runBuiltinEditor(path)
}
```

- [ ] **Step 5: Handle errQuitWithoutSaving in cmdNew**

In `cmdNew`, the current error check (around line 348) is:

```go
if err := openInEditor(tmpPath); err != nil {
    fmt.Fprintln(os.Stderr, "editor error:", err)
    return
}
```

Replace with:

```go
if err := openInEditor(tmpPath); err != nil {
    if errors.Is(err, errQuitWithoutSaving) {
        fmt.Println("  Nothing written — discarded.")
        return
    }
    fmt.Fprintln(os.Stderr, "editor error:", err)
    return
}
```

Also add `"errors"` to the import block at the top of `main.go` if not already present.

- [ ] **Step 6: Build and verify**

```bash
cd /home/dcb/jnlport && go build ./...
```

Expected: no output, exit 0.

- [ ] **Step 7: Run all tests**

```bash
go test ./... -v
```

Expected: all tests PASS.

- [ ] **Step 8: Smoke test on this machine**

```bash
# Temporarily unset EDITOR so the built-in fires
EDITOR= ./jnl-go new "test entry"
```

Expected: built-in editor opens full-screen. Type something, Ctrl+S saves. Confirm "Draft saved. Inbox: N draft(s)." appears.

Also test quit-without-saving:

```bash
EDITOR= ./jnl-go new
```

Open, immediately Ctrl+Q (or Esc Esc). Expected: "Nothing written — discarded."

- [ ] **Step 9: Commit**

```bash
git add main.go editor_test.go
git commit -m "feat: wire built-in editor into openInEditor with micro fallback"
```

---

## Task 4: Cross-platform build check

**Files:**
- Modify: `Makefile` (verify existing cross-compile targets still work)

- [ ] **Step 1: Check the Makefile**

```bash
cat /home/dcb/jnlport/Makefile
```

Note the existing cross-compile targets.

- [ ] **Step 2: Build for Windows amd64**

```bash
cd /home/dcb/jnlport && GOOS=windows GOARCH=amd64 go build -o dist/jnl.exe .
```

Expected: `dist/jnl.exe` created, no errors.

- [ ] **Step 3: Build for macOS arm64**

```bash
GOOS=darwin GOARCH=arm64 go build -o dist/jnl-macos-arm64 .
```

Expected: binary created, no errors.

- [ ] **Step 4: Commit dist binaries (if tracked) or just confirm clean build**

```bash
git status
```

If `dist/` is tracked:

```bash
git add dist/
git commit -m "build: cross-compile with built-in editor"
```

If `dist/` is in `.gitignore`, no commit needed.
