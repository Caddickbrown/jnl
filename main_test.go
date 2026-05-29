package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempNotesDir points the package-level path globals at a fresh temp dir
// for the duration of a test, restoring them afterwards.
func withTempNotesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	prevNotes, prevInbox, prevTransfer, prevJournal := notesDir, inboxPath, transferPath, journalDir
	notesDir = dir
	inboxPath = filepath.Join(dir, "inbox.md")
	transferPath = filepath.Join(dir, "transfer.md")
	journalDir = filepath.Join(dir, "journal")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		notesDir, inboxPath, transferPath, journalDir = prevNotes, prevInbox, prevTransfer, prevJournal
	})
	return dir
}

// ── parseEntries ────────────────────────────────────────────────────────────

func TestParseEntries_BasicFields(t *testing.T) {
	content := "[2026-05-10 08:00:00] A title\nfirst line\nsecond line\n"
	entries := parseEntries(content)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.date != "2026-05-10" {
		t.Errorf("date = %q, want 2026-05-10", e.date)
	}
	if e.ts != "2026-05-10 08:00:00" {
		t.Errorf("ts = %q", e.ts)
	}
	if e.suffix != "A title" {
		t.Errorf("suffix = %q, want %q", e.suffix, "A title")
	}
	if e.body != "first line\nsecond line" {
		t.Errorf("body = %q", e.body)
	}
}

func TestParseEntries_MultipleAndPreambleDropped(t *testing.T) {
	content := "orphan line\n\n[2026-05-10 08:00:00] one\nbody1\n\n[2026-05-11 09:00:00] two\nbody2\n"
	entries := parseEntries(content)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// parseEntries intentionally drops preamble; leadingPreamble is what
	// rewrite paths use to preserve it.
	if entries[0].suffix != "one" || entries[1].suffix != "two" {
		t.Errorf("unexpected suffixes: %q, %q", entries[0].suffix, entries[1].suffix)
	}
}

// ── leadingPreamble ───────────────────────────────────────────────────────────

func TestLeadingPreamble(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"none", "[2026-05-10 08:00:00] x\nbody\n", ""},
		{"blank only", "\n\n[2026-05-10 08:00:00] x\n", ""},
		{"present", "loose note\n\n[2026-05-10 08:00:00] x\n", "loose note"},
		{"multiline", "a\nb\n[2026-05-10 08:00:00] x\n", "a\nb"},
		{"no headers at all", "just text\nmore text\n", "just text\nmore text"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := leadingPreamble(c.content); got != c.want {
				t.Errorf("leadingPreamble(%q) = %q, want %q", c.content, got, c.want)
			}
		})
	}
}

// ── sortedByTimestamp ─────────────────────────────────────────────────────────

func TestSortedByTimestamp_OrdersOldestFirst(t *testing.T) {
	entries := []entry{
		{ts: "2026-05-20 09:00:00", suffix: "c"},
		{ts: "2026-05-10 08:00:00", suffix: "a"},
		{ts: "2026-05-15 12:30:00", suffix: "b"},
	}
	got := sortedByTimestamp(entries)
	want := []string{"a", "b", "c"}
	for i, w := range want {
		if got[i].suffix != w {
			t.Errorf("position %d = %q, want %q", i, got[i].suffix, w)
		}
	}
	// Original slice must be untouched.
	if entries[0].suffix != "c" {
		t.Errorf("input slice was mutated: %q", entries[0].suffix)
	}
}

func TestSortedByTimestamp_StableForEqualTimestamps(t *testing.T) {
	entries := []entry{
		{ts: "2026-05-10 08:00:00", suffix: "first"},
		{ts: "2026-05-10 08:00:00", suffix: "second"},
		{ts: "2026-05-10 08:00:00", suffix: "third"},
	}
	got := sortedByTimestamp(entries)
	want := []string{"first", "second", "third"}
	for i, w := range want {
		if got[i].suffix != w {
			t.Errorf("position %d = %q, want %q", i, got[i].suffix, w)
		}
	}
}

// ── splitTagTarget (whole-token matching) ─────────────────────────────────────

func TestSplitTagTarget_WholeTokenOnly(t *testing.T) {
	dir := withTempNotesDir(t)
	t.Setenv("JNL_SPLIT_TAGS", "@work")

	// Substring of a split tag must NOT route.
	if tag, path := splitTagTarget("[2026-05-20 09:00:00] about the @workshop"); path != "" {
		t.Errorf("@workshop wrongly routed: tag=%q path=%q", tag, path)
	}
	// Exact tag token must route to <notesDir>/work.md.
	tag, path := splitTagTarget("[2026-05-20 09:00:00] real @work item")
	if tag != "@work" {
		t.Errorf("tag = %q, want @work", tag)
	}
	if want := filepath.Join(dir, "work.md"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

func TestSplitTagTarget_NoTagsConfigured(t *testing.T) {
	withTempNotesDir(t)
	t.Setenv("JNL_SPLIT_TAGS", "")
	if _, path := splitTagTarget("[2026-05-20 09:00:00] @work thing"); path != "" {
		t.Errorf("expected no routing when JNL_SPLIT_TAGS empty, got %q", path)
	}
}

// ── rebuildInbox preamble preservation ────────────────────────────────────────

func TestRebuildInbox_PreservesPreamble(t *testing.T) {
	withTempNotesDir(t)
	entries := []entry{
		{ts: "2026-05-10 08:00:00", raw: "[2026-05-10 08:00:00] a\nbody"},
	}
	if err := rebuildInbox("loose note", entries); err != nil {
		t.Fatal(err)
	}
	got := readFile(inboxPath)
	want := "loose note\n\n[2026-05-10 08:00:00] a\nbody\n"
	if got != want {
		t.Errorf("rebuildInbox output = %q, want %q", got, want)
	}
}

func TestRebuildInbox_EmptyClearsFile(t *testing.T) {
	withTempNotesDir(t)
	writeFile(inboxPath, "stale content")
	if err := rebuildInbox("", nil); err != nil {
		t.Fatal(err)
	}
	if got := readFile(inboxPath); got != "" {
		t.Errorf("expected empty inbox, got %q", got)
	}
}

// ── cmdSort end-to-end ────────────────────────────────────────────────────────

func TestCmdSort_OrdersInPlaceKeepingPreamble(t *testing.T) {
	withTempNotesDir(t)
	writeFile(inboxPath,
		"loose preamble\n\n"+
			"[2026-05-20 09:00:00] later\nB1\n\n"+
			"[2026-05-10 08:00:00] earlier\nB2\n")

	cmdSort()

	got := readFile(inboxPath)
	want := "loose preamble\n\n" +
		"[2026-05-10 08:00:00] earlier\nB2\n\n" +
		"[2026-05-20 09:00:00] later\nB1\n"
	if got != want {
		t.Errorf("after sort = %q\nwant %q", got, want)
	}
}

// ── fileDraft routing ─────────────────────────────────────────────────────────

func TestFileDraft_DateRoutedToJournal(t *testing.T) {
	dir := withTempNotesDir(t)
	e := entry{
		date:   "2026-05-10",
		ts:     "2026-05-10 08:00:00",
		suffix: "title",
		body:   "the body",
		raw:    "[2026-05-10 08:00:00] title\nthe body",
	}
	label, err := fileDraft(e)
	if err != nil {
		t.Fatal(err)
	}
	if label != "2026-05-10" {
		t.Errorf("label = %q, want 2026-05-10", label)
	}
	jfile := filepath.Join(dir, "journal", "2026", "05", "10.md")
	if !fileExists(jfile) {
		t.Fatalf("expected journal file at %s", jfile)
	}
}

func TestFileDraft_SplitTagRouted(t *testing.T) {
	dir := withTempNotesDir(t)
	t.Setenv("JNL_SPLIT_TAGS", "@work")
	e := entry{
		date: "2026-05-10",
		ts:   "2026-05-10 08:00:00",
		body: "a @work note",
		raw:  "[2026-05-10 08:00:00]\na @work note",
	}
	label, err := fileDraft(e)
	if err != nil {
		t.Fatal(err)
	}
	if label != "@work" {
		t.Errorf("label = %q, want @work", label)
	}
	if !fileExists(filepath.Join(dir, "work.md")) {
		t.Errorf("expected work.md to be created")
	}
}

func TestFileDraft_EmptyEntryErrors(t *testing.T) {
	withTempNotesDir(t)
	if _, err := fileDraft(entry{ts: "2026-05-10 08:00:00"}); err == nil {
		t.Error("expected error filing an empty entry")
	}
}

// ── atomic writeFile ──────────────────────────────────────────────────────────

func TestWriteFile_RoundTripAndMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")
	if err := writeFile(path, "hello\n"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(path); got != "hello\n" {
		t.Errorf("content = %q", got)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0644 {
		t.Errorf("mode = %v, want 0644", fi.Mode().Perm())
	}
	// Overwriting leaves no stray temp files behind.
	if err := writeFile(path, "again\n"); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".jnl-*.tmp"))
	if len(matches) != 0 {
		t.Errorf("temp files left behind: %v", matches)
	}
}

// ── cmdCleanup split-tag extraction ─────────────────────────────────────────

func TestCmdCleanup_ExtractsSplitTaggedEntries(t *testing.T) {
	dir := withTempNotesDir(t)
	t.Setenv("JNL_SPLIT_TAGS", "@work")

	// A date journal with one plain entry and one tagged entry.
	jf := journalFile("2026-05-20")
	if err := os.MkdirAll(filepath.Dir(jf), 0755); err != nil {
		t.Fatal(err)
	}
	content := "[2026-05-20 08:00:00] plain note\nbody one\n\n" +
		"[2026-05-20 09:00:00] @work standup\nbody two\n"
	if err := writeFile(jf, content); err != nil {
		t.Fatal(err)
	}

	cmdCleanup()

	// The tagged entry must have moved out of the date journal...
	dateFile := readFile(jf)
	if !strings.Contains(dateFile, "plain note") {
		t.Errorf("date journal lost plain entry: %q", dateFile)
	}
	if strings.Contains(dateFile, "@work standup") {
		t.Errorf("tagged entry was not removed from date journal: %q", dateFile)
	}

	// ...and into work.md.
	tagFile := readFile(filepath.Join(dir, "work.md"))
	if !strings.Contains(tagFile, "@work standup") || !strings.Contains(tagFile, "body two") {
		t.Errorf("tagged entry not filed into work.md: %q", tagFile)
	}
}
