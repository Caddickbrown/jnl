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
	saveErr   string
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
			if err := os.WriteFile(m.path, []byte(m.textarea.Value()), 0644); err != nil {
				m.saveErr = "Save failed: " + err.Error()
				return m, nil
			}
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
			m.escCount = 0
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
	hint := "Ctrl+S save · Ctrl+Z undo · Ctrl+Q quit · Esc×2 quit"
	if m.saveErr != "" {
		hint = m.saveErr + " — " + hint
	}
	return m.textarea.View() + "\n\n" + hint
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
	if fm, ok := final.(editorModel); !ok || fm.status == editorQuit {
		return errQuitWithoutSaving
	}
	return nil
}
