# Built-in Editor Design

**Date:** 2026-04-09  
**Status:** Approved

## Problem

jnl currently delegates all text editing to an external `$EDITOR` (defaulting to `micro`). On Windows — especially locked-down corporate machines — no terminal text editor is reliably available. The goal is a self-contained built-in editor that works on Windows, macOS, and Linux with no external dependencies.

## Approach

Use `github.com/charmbracelet/bubbletea` and `github.com/charmbracelet/bubbles/textarea` to build a minimal but fully functional terminal editor embedded in the jnl binary.

## Architecture

A new file `editor.go` in the `main` package exposes one function:

```go
func runBuiltinEditor(path string) error
```

It takes a file path, blocks while the user edits, and returns an error (or a sentinel quit-without-saving error). This matches the existing `openInEditor` contract exactly — the rest of `main.go` is unchanged except for the fallback logic in `openInEditor`.

The bubbletea model holds:
- `textarea.Model` — the editing component
- `undoStack []string` — snapshots of textarea content, capped at 100
- `path string` — file being edited
- `status` — one of `editing`, `saved`, `quit`

## Fallback Logic

`openInEditor` resolves the editor in this order:

1. `EDITOR` explicitly set in environment → use it unconditionally
2. `EDITOR` unset → use built-in editor
3. `EDITOR` is `micro` (compiled-in default) and `micro` is not on PATH → use built-in editor

The built-in is never used when the user has explicitly configured an external editor.

## Key Bindings

| Key | Action |
|-----|--------|
| `Ctrl+S` | Save and exit |
| `Ctrl+Q` or `Esc Esc` | Quit without saving (confirm if dirty) |
| `Ctrl+Z` | Undo |
| Arrow keys | Cursor movement |
| `Home` / `End` | Start / end of line |
| `Ctrl+←` / `Ctrl+→` | Word jump |
| `Backspace` / `Delete` | Delete character |

Arrow keys, Home/End, Backspace/Delete, and word-jump are handled natively by the `bubbles/textarea` component. Undo and save/quit are handled by the bubbletea model.

## Undo Stack

On every content-changing keypress, the current textarea value is pushed onto `undoStack`. The stack is capped at 100 entries. `Ctrl+Z` pops and restores. This is reliable for journal entry lengths.

## Cursor Position

On open, the cursor is placed at the end of the existing content — matching the `+99999` behaviour used with micro today.

## Error Handling

- Built-in editor fails to initialise (e.g. no TTY) → return error, caller prints and bails
- User quits without saving → return a sentinel `errQuitWithoutSaving` so `cmdNew` / `cmdReview` discard the temp file, same as the existing "nothing written" path
- Save is a direct write to the temp file path; `cmdNew` and `cmdReview` already read it back after `openInEditor` returns

## Syntax Highlighting

Not in scope for this version. The architecture does not preclude adding it later — the bubbletea model's `View()` method renders the textarea, and a highlighting pass could be applied there.

## Out of Scope

- Copy/paste (system clipboard integration is complex cross-platform)
- Mouse support
- Syntax highlighting (deferred)
- Any UI beyond a plain editing area and a one-line status bar showing key hints
