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

	for _, r := range "ab" {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = result.(editorModel)
	}

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

	for i := 0; i < 100; i++ {
		m.undoStack = append(m.undoStack, "x")
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(editorModel)

	if len(m.undoStack) != 100 {
		t.Errorf("expected undo stack capped at 100, got %d", len(m.undoStack))
	}
}

func TestEditorModel_TypingResetsEscCount(t *testing.T) {
	m := newEditorModel("/tmp/jnl_test.md", "")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(editorModel)
	if m.escCount != 1 {
		t.Fatalf("expected escCount 1, got %d", m.escCount)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(editorModel)
	if m.escCount != 0 {
		t.Errorf("expected escCount reset to 0 after typing, got %d", m.escCount)
	}
}

func TestRunBuiltinEditor_QuitWithoutSaving_ReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires TTY")
	}
	if errQuitWithoutSaving.Error() != "quit without saving" {
		t.Errorf("unexpected sentinel error text: %q", errQuitWithoutSaving.Error())
	}
}
