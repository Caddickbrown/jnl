// jnl — a terminal journaling toolkit (Go port)
//
// Identical file format to the bash version:
//   $JNL_DIR/inbox.md          — unsorted drafts
//   $JNL_DIR/journal/YYYY/MM/DD.md — filed entries
//
// Build:
//   go build -o jnl .
//
// Cross-compile (see Makefile):
//   make all
package main

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// ── config ────────────────────────────────────────────────────────────────────

var (
	notesDir   string
	inboxPath  string
	journalDir string
)

func init() {
	home, _ := os.UserHomeDir()
	notesDir = envOr("JNL_DIR", filepath.Join(home, "notes"))
	inboxPath = filepath.Join(notesDir, "inbox.md")
	journalDir = filepath.Join(notesDir, "journal")
	os.MkdirAll(journalDir, 0755)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ── time helpers ──────────────────────────────────────────────────────────────

func nowTS() string   { return time.Now().Format("2006-01-02 15:04:05") }
func nowDate() string { return time.Now().Format("2006-01-02") }

func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.Local)
}

func yesterdayDate() string {
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

// ── file path helpers ─────────────────────────────────────────────────────────

func journalFile(dateStr string) string {
	parts := strings.SplitN(dateStr, "-", 3)
	if len(parts) != 3 {
		return ""
	}
	return filepath.Join(journalDir, parts[0], parts[1], parts[2]+".md")
}

func fileToDate(path string) string {
	day := strings.TrimSuffix(filepath.Base(path), ".md")
	month := filepath.Base(filepath.Dir(path))
	year := filepath.Base(filepath.Dir(filepath.Dir(path)))
	return year + "-" + month + "-" + day
}

// ── file I/O ──────────────────────────────────────────────────────────────────

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

func wordCountFile(path string) int {
	return wordCount(readFile(path))
}

// ── journal file discovery ────────────────────────────────────────────────────

func allJournalFiles(ascending bool) []string {
	var files []string
	filepath.Walk(journalDir, func(path string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	if !ascending {
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}
	return files
}

// ── entry parsing ─────────────────────────────────────────────────────────────

var entryHeaderRE = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2})\](.*)`)

type entry struct {
	date   string // YYYY-MM-DD
	ts     string // YYYY-MM-DD HH:MM:SS
	suffix string // text after timestamp on header line
	body   string // lines after header, stripped of leading/trailing blank lines
	raw    string // full original text including header line
}

func parseEntries(content string) []entry {
	var entries []entry
	var cur []string
	var curTS, curDate, curSuffix string

	flush := func() {
		if curTS == "" {
			return
		}
		body := strings.Join(cur[1:], "\n")
		body = strings.TrimSpace(body)
		entries = append(entries, entry{
			date:   curDate,
			ts:     curTS,
			suffix: curSuffix,
			body:   body,
			raw:    strings.Join(cur, "\n"),
		})
	}

	for _, line := range strings.Split(content, "\n") {
		if m := entryHeaderRE.FindStringSubmatch(line); m != nil {
			flush()
			curDate = m[1]
			curTS = m[1] + " " + m[2]
			curSuffix = strings.TrimSpace(m[3])
			cur = []string{line}
		} else if curTS != "" {
			cur = append(cur, line)
		}
	}
	flush()
	return entries
}

func countEntries(path string) int {
	content := readFile(path)
	return len(parseEntries(content))
}

// ── inbox helpers ─────────────────────────────────────────────────────────────

// countDrafts counts timestamp header lines in inbox
func countDrafts() int {
	if !fileNonEmpty(inboxPath) {
		return 0
	}
	entries := parseEntries(readFile(inboxPath))
	return len(entries)
}

// splitInbox parses the inbox into individual entry structs
func splitInbox() []entry {
	if !fileNonEmpty(inboxPath) {
		return nil
	}
	return parseEntries(readFile(inboxPath))
}

// rebuildInbox writes a slice of entries back to inbox
func rebuildInbox(entries []entry) error {
	if len(entries) == 0 {
		return os.WriteFile(inboxPath, nil, 0644)
	}
	var parts []string
	for _, e := range entries {
		// Rebuild from raw, normalised to one trailing newline
		parts = append(parts, strings.TrimRight(e.raw, "\n"))
	}
	content := strings.Join(parts, "\n\n") + "\n"
	return writeFile(inboxPath, content)
}

// fileDraft appends a draft entry to the correct journal day file
func fileDraft(e entry) (string, error) {
	date := e.date
	if date == "" {
		date = nowDate()
	}
	if e.body == "" {
		return "", fmt.Errorf("nothing to file")
	}

	jfile := journalFile(date)
	if err := os.MkdirAll(filepath.Dir(jfile), 0755); err != nil {
		return "", err
	}

	var sb strings.Builder
	if fileNonEmpty(jfile) {
		sb.WriteString(readFile(jfile))
		if !strings.HasSuffix(sb.String(), "\n\n") {
			if strings.HasSuffix(sb.String(), "\n") {
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n\n")
			}
		}
	}

	if e.suffix != "" {
		fmt.Fprintf(&sb, "[%s] %s\n%s\n", e.ts, e.suffix, e.body)
	} else {
		fmt.Fprintf(&sb, "[%s]\n%s\n", e.ts, e.body)
	}

	return date, writeFile(jfile, sb.String())
}

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

// ── terminal raw input ────────────────────────────────────────────────────────

type keyEvent struct {
	char byte
	seq  string // "up", "down", "left", "right", "backspace", "enter", "esc"
}

func readKey() keyEvent {
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		// fallback: line-buffered read
		var b [1]byte
		os.Stdin.Read(b[:])
		return keyEvent{char: b[0]}
	}
	defer term.Restore(fd, old)

	buf := make([]byte, 1)
	os.Stdin.Read(buf)

	if buf[0] == 27 { // ESC
		seq := make([]byte, 2)
		n, _ := os.Stdin.Read(seq)
		if n == 2 && seq[0] == '[' {
			switch seq[1] {
			case 'A':
				return keyEvent{seq: "up"}
			case 'B':
				return keyEvent{seq: "down"}
			case 'C':
				return keyEvent{seq: "right"}
			case 'D':
				return keyEvent{seq: "left"}
			}
		}
		return keyEvent{seq: "esc"}
	}
	if buf[0] == 13 {
		return keyEvent{seq: "enter"}
	}
	if buf[0] == 127 || buf[0] == 8 {
		return keyEvent{seq: "backspace"}
	}
	return keyEvent{char: buf[0]}
}

func clearScreen() { fmt.Print("\033[H\033[2J") }

// ── commands ──────────────────────────────────────────────────────────────────

func cmdNew(title string) {
	count := countDrafts()
	if fileNonEmpty(inboxPath) {
		fmt.Printf("  Inbox: %d draft(s) waiting.\n", count)
	} else {
		fmt.Println("  Inbox: empty.")
	}

	tmp, err := os.CreateTemp("", "jnl_new_*.md")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	ts := nowTS()
	if title != "" {
		fmt.Fprintf(tmp, "[%s] %s\n", ts, title)
	} else {
		fmt.Fprintf(tmp, "[%s]\n", ts)
	}
	tmp.Close()

	if err := openInEditor(tmpPath); err != nil {
		if errors.Is(err, errQuitWithoutSaving) {
			fmt.Println("  Nothing written — discarded.")
			return
		}
		fmt.Fprintln(os.Stderr, "editor error:", err)
		return
	}

	raw := readFile(tmpPath)
	entries := parseEntries(raw)
	if len(entries) == 0 || strings.TrimSpace(entries[0].body) == "" {
		fmt.Println("  Nothing written — discarded.")
		return
	}

	// Normalise: strip trailing newlines, add exactly one
	normalised := strings.TrimRight(raw, "\n") + "\n"

	// Append to inbox
	if fileNonEmpty(inboxPath) {
		existing := strings.TrimRight(readFile(inboxPath), "\n")
		normalised = existing + "\n\n" + normalised
	}
	writeFile(inboxPath, normalised)

	fmt.Printf("  Draft saved. Inbox: %d draft(s).\n", countDrafts())
}

func cmdReview() {
	if !fileNonEmpty(inboxPath) {
		fmt.Println("  Inbox is empty — nothing to review.")
		return
	}

	drafts := splitInbox()
	var filed, deleted int

	idx := 0
	for {
		if len(drafts) == 0 {
			break
		}
		if idx >= len(drafts) {
			idx = len(drafts) - 1
		}
		if idx < 0 {
			idx = 0
		}

		d := drafts[idx]
		wc := wordCount(d.body)

		clearScreen()
		fmt.Printf("\n  ─── Draft %d of %d", idx+1, len(drafts))
		if d.ts != "" {
			fmt.Printf("  ·  %s", d.ts)
		}
		fmt.Printf("  ·  %d words ───\n\n", wc)

		lines := strings.Split(d.body, "\n")
		for i, l := range lines {
			if i >= 25 {
				break
			}
			fmt.Printf("  %s\n", l)
		}
		fmt.Println()
		fmt.Print("  ↑/↓ navigate  [f]ile  [e]dit  [d]elete  [q]uit\n  > ")

		k := readKey()
		fmt.Println()

		switch {
		case k.seq == "up":
			idx--
			if idx < 0 {
				idx = len(drafts) - 1
			}
			continue
		case k.seq == "down":
			idx++
			if idx >= len(drafts) {
				idx = 0
			}
			continue
		case k.char == 'f' || k.char == 'F':
			date, err := fileDraft(d)
			if err != nil {
				fmt.Println(" ", err)
			} else {
				fmt.Printf("  → %s.md\n", date)
				drafts = append(drafts[:idx], drafts[idx+1:]...)
				filed++
				if idx >= len(drafts) {
					idx = len(drafts) - 1
				}
			}
		case k.char == 'e' || k.char == 'E':
			// Write draft to temp, edit, re-parse
			tmp, _ := os.CreateTemp("", "jnl_edit_*.md")
			tmpPath := tmp.Name()
			fmt.Fprint(tmp, d.raw)
			tmp.Close()
			if err := openInEditor(tmpPath); err != nil {
				os.Remove(tmpPath)
				if errors.Is(err, errQuitWithoutSaving) {
					fmt.Println("  Edit discarded.")
				} else {
					fmt.Fprintln(os.Stderr, "editor error:", err)
				}
				break
			}
			raw := readFile(tmpPath)
			os.Remove(tmpPath)
			parsed := parseEntries(raw)
			if len(parsed) > 0 {
				drafts[idx] = parsed[0]
			}
		case k.char == 'd' || k.char == 'D':
			fmt.Print("  Delete this draft? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			ans, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(ans)) == "y" {
				drafts = append(drafts[:idx], drafts[idx+1:]...)
				deleted++
				fmt.Println("  Deleted.")
				if idx >= len(drafts) {
					idx = len(drafts) - 1
				}
			}
		case k.char == 'q' || k.char == 'Q':
			fmt.Println("  Stopped.")
			goto done
		}
	}

done:
	rebuildInbox(drafts)
	remaining := len(drafts)

	fmt.Println()
	fmt.Println("  ── Done ─────────────────")
	fmt.Printf("  Filed:     %d\n", filed)
	fmt.Printf("  Deleted:   %d\n", deleted)
	fmt.Printf("  Remaining: %d\n", remaining)
	fmt.Println()
}

func cmdBrowse() {
	files := allJournalFiles(true)
	if len(files) == 0 {
		fmt.Println("  No journal entries yet.")
		return
	}

	level := "year"
	var selYear, selMonth string
	idx := 0
	filter := ""

	for {
		var items []string
		switch level {
		case "year":
			seen := map[string]bool{}
			for _, f := range files {
				y := filepath.Base(filepath.Dir(filepath.Dir(f)))
				if !seen[y] {
					seen[y] = true
					items = append(items, y)
				}
			}
			sort.Sort(sort.Reverse(sort.StringSlice(items)))
		case "month":
			seen := map[string]bool{}
			for _, f := range files {
				y := filepath.Base(filepath.Dir(filepath.Dir(f)))
				m := filepath.Base(filepath.Dir(f))
				if y == selYear && !seen[m] {
					seen[m] = true
					items = append(items, m)
				}
			}
			sort.Sort(sort.Reverse(sort.StringSlice(items)))
		case "day":
			for _, f := range files {
				y := filepath.Base(filepath.Dir(filepath.Dir(f)))
				m := filepath.Base(filepath.Dir(f))
				if y == selYear && m == selMonth {
					items = append(items, strings.TrimSuffix(filepath.Base(f), ".md"))
				}
			}
			sort.Sort(sort.Reverse(sort.StringSlice(items)))
		}

		// Apply filter
		var filtered []string
		if filter != "" {
			for _, item := range items {
				if strings.HasPrefix(item, filter) {
					filtered = append(filtered, item)
				}
			}
			if len(filtered) == 0 {
				if len(filter) > 0 {
					filter = filter[:len(filter)-1]
				}
				continue
			}
		} else {
			filtered = items
		}

		if idx >= len(filtered) {
			idx = len(filtered) - 1
		}
		if idx < 0 {
			idx = 0
		}

		clearScreen()
		switch level {
		case "year":
			fmt.Printf("\n  ── Journal ──────────────────────────\n\n")
		case "month":
			fmt.Printf("\n  ── %s ───────────────────────────────\n\n", selYear)
		case "day":
			t, _ := time.ParseInLocation("2006-01", selYear+"-"+selMonth, time.Local)
			fmt.Printf("\n  ── %s / %s ──────────────────────────\n\n", selYear, t.Format("January"))
		}

		for i, item := range filtered {
			var label string
			switch level {
			case "year":
				count := 0
				for _, f := range files {
					if filepath.Base(filepath.Dir(filepath.Dir(f))) == item {
						count++
					}
				}
				label = fmt.Sprintf("%s  (%d day(s))", item, count)
			case "month":
				count := 0
				for _, f := range files {
					y := filepath.Base(filepath.Dir(filepath.Dir(f)))
					m := filepath.Base(filepath.Dir(f))
					if y == selYear && m == item {
						count++
					}
				}
				t, _ := time.ParseInLocation("2006-01", selYear+"-"+item, time.Local)
				label = fmt.Sprintf("%s  (%d day(s))", t.Format("January"), count)
			case "day":
				dateStr := selYear + "-" + selMonth + "-" + item
				jfile := journalFile(dateStr)
				ec := countEntries(jfile)
				wc := wordCountFile(jfile)
				t, _ := parseDate(dateStr)
				entWord := "entries"
				if ec == 1 {
					entWord = "entry"
				}
				label = fmt.Sprintf("%s  %s  —  %d %s, %d words",
					item, t.Format("Mon"), ec, entWord, wc)
			}

			if i == idx {
				fmt.Printf("  ▶ %s\n", label)
			} else {
				fmt.Printf("    %s\n", label)
			}
		}

		fmt.Println()
		if filter != "" {
			fmt.Printf("  Filter: %s\n", filter)
		}
		switch level {
		case "year":
			fmt.Print("  ↑/↓ navigate  Enter select  q quit\n  > ")
		case "month":
			fmt.Print("  ↑/↓ navigate  Enter select  ← back  q quit\n  > ")
		case "day":
			fmt.Print("  ↑/↓ navigate  Enter view  ← back  q quit\n  > ")
		}

		k := readKey()

		switch k.seq {
		case "up":
			filter = ""
			idx--
			if idx < 0 {
				idx = len(filtered) - 1
			}
			continue
		case "down":
			filter = ""
			idx++
			if idx >= len(filtered) {
				idx = 0
			}
			continue
		case "left":
			switch level {
			case "month":
				level = "year"
				selYear = ""
				idx = 0
				filter = ""
			case "day":
				level = "month"
				selMonth = ""
				idx = 0
				filter = ""
			}
			continue
		case "enter":
			selected := filtered[idx]
			switch level {
			case "year":
				selYear = selected
				level = "month"
				idx = 0
				filter = ""
			case "month":
				selMonth = selected
				level = "day"
				idx = 0
				filter = ""
			case "day":
				fmt.Println()
				cmdLog(selYear + "-" + selMonth + "-" + selected)
				fmt.Print("  Press any key to continue...")
				readKey()
			}
			continue
		case "backspace":
			if filter != "" {
				if len(filter) > 0 {
					// trim last rune
					_, size := utf8.DecodeLastRuneInString(filter)
					filter = filter[:len(filter)-size]
				}
				idx = 0
			} else {
				switch level {
				case "month":
					level = "year"
					selYear = ""
					idx = 0
				case "day":
					level = "month"
					selMonth = ""
					idx = 0
				case "year":
					clearScreen()
					return
				}
			}
			continue
		}

		switch k.char {
		case 'q', 'Q':
			clearScreen()
			return
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			filter += string(k.char)
			idx = 0
		}
	}
}

func cmdInbox() {
	if !fileNonEmpty(inboxPath) {
		fmt.Println("  Inbox is empty.")
		return
	}
	count := countDrafts()
	fmt.Println()
	fmt.Printf("  ── Inbox: %d draft(s) ──────────────\n", count)
	content := readFile(inboxPath)
	for _, line := range strings.Split(content, "\n") {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println()
}

func cmdLog(dateStr string) {
	if dateStr == "" {
		dateStr = nowDate()
	}
	jfile := journalFile(dateStr)
	if !fileExists(jfile) {
		fmt.Printf("  No entries for %s.\n", dateStr)
		return
	}
	ec := countEntries(jfile)
	wc := wordCountFile(jfile)
	entWord := "entries"
	if ec == 1 {
		entWord = "entry"
	}
	fmt.Println()
	fmt.Printf("  %s — %d %s, %d words\n", dateStr, ec, entWord, wc)
	fmt.Println("  ──────────────────────────────")
	content := readFile(jfile)
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println()
}

func cmdYesterday() { cmdLog(yesterdayDate()) }

func cmdList() {
	files := allJournalFiles(false)
	fmt.Println()
	if len(files) == 0 {
		fmt.Println("  No journal entries yet.")
		fmt.Println()
		return
	}
	var totalWords int
	for _, f := range files {
		dateStr := fileToDate(f)
		ec := countEntries(f)
		wc := wordCountFile(f)
		totalWords += wc
		fmt.Printf("  %-14s  %2d entries  %4d words\n", dateStr, ec, wc)
	}
	fmt.Println("  ───────────────────────────────────")
	fmt.Printf("  %d days  ·  %d words total\n", len(files), totalWords)
	fmt.Println()
}

func cmdStats() {
	files := allJournalFiles(true)
	fmt.Println()
	fmt.Println("  Writing stats")
	fmt.Println("  ─────────────────────────────────────────────────")

	var totalWords, totalEntries int
	var longestWC int
	var longestDate string
	var busiestEC int
	var busiestDate string
	var streak, maxStreak int
	var streakStart, maxStreakStart, maxStreakEnd string
	var prevDate time.Time
	prevDateStr := ""

	yearEntries := map[string]int{}
	yearWords := map[string]int{}
	dowCount := map[int]int{}
	allDates := []time.Time{}

	for _, f := range files {
		wc := wordCountFile(f)
		ec := countEntries(f)
		dateStr := fileToDate(f)
		t, err := parseDate(dateStr)
		if err != nil {
			continue
		}

		totalWords += wc
		totalEntries += ec
		allDates = append(allDates, t)

		yr := strconv.Itoa(t.Year())
		yearEntries[yr] += ec
		yearWords[yr] += wc
		dow := int(t.Weekday()) // 0=Sun…6=Sat; we want 1=Mon…7=Sun
		if dow == 0 {
			dow = 7
		}
		dowCount[dow]++

		if wc > longestWC {
			longestWC = wc
			longestDate = dateStr
		}
		if ec > busiestEC {
			busiestEC = ec
			busiestDate = dateStr
		}

		if prevDateStr != "" {
			if t.Equal(prevDate.AddDate(0, 0, 1)) {
				streak++
				if streak > maxStreak {
					maxStreak = streak
					maxStreakStart = streakStart
					maxStreakEnd = dateStr
				}
			} else {
				streak = 1
				streakStart = dateStr
			}
		} else {
			streak = 1
			streakStart = dateStr
			maxStreak = 1
			maxStreakStart = dateStr
			maxStreakEnd = dateStr
		}
		prevDate = t
		prevDateStr = dateStr
	}

	// Current streak (backwards from today)
	today := time.Now().Truncate(24 * time.Hour)
	currStreak := 0
	currStreakStart := ""
	for i := len(allDates) - 1; i >= 0; i-- {
		expected := today.AddDate(0, 0, -currStreak)
		if allDates[i].Equal(expected) {
			currStreak++
			currStreakStart = allDates[i].Format("2006-01-02")
		} else {
			break
		}
	}

	inboxCount := countDrafts()
	days := len(files)

	fmt.Printf("  Days written:     %d\n", days)
	fmt.Printf("  Total entries:    %d\n", totalEntries)
	fmt.Printf("  Total words:      %d\n", totalWords)
	if totalEntries > 0 {
		fmt.Printf("  Avg per entry:    %d words\n", totalWords/totalEntries)
	}
	if longestDate != "" {
		fmt.Printf("  Longest day:      %s  (%d words)\n", longestDate, longestWC)
	}
	if busiestEC > 1 {
		fmt.Printf("  Busiest day:      %s  (%d entries)\n", busiestDate, busiestEC)
	}
	fmt.Printf("  Inbox drafts:     %d waiting\n", inboxCount)
	fmt.Println()

	fmt.Printf("  Current streak:   %d day(s)", currStreak)
	if currStreak > 1 {
		fmt.Printf("  (%s → %s)", currStreakStart, nowDate())
	}
	fmt.Println()
	fmt.Printf("  Best streak:      %d day(s)", maxStreak)
	if maxStreak > 1 {
		fmt.Printf("  (%s → %s)", maxStreakStart, maxStreakEnd)
	}
	fmt.Println()
	fmt.Println()

	// Per-year breakdown
	if len(yearEntries) > 0 {
		var years []string
		for yr := range yearEntries {
			years = append(years, yr)
		}
		sort.Strings(years)
		fmt.Printf("  %-6s  %8s  %8s\n", "Year", "Entries", "Words")
		fmt.Println("  ──────────────────────────────")
		for _, yr := range years {
			fmt.Printf("  %-6s  %8d  %8d\n", yr, yearEntries[yr], yearWords[yr])
		}
		fmt.Println()
	}

	// Day-of-week breakdown
	dowNames := map[int]string{1: "Mon", 2: "Tue", 3: "Wed", 4: "Thu", 5: "Fri", 6: "Sat", 7: "Sun"}
	hasAny := false
	for d := 1; d <= 7; d++ {
		if dowCount[d] > 0 {
			hasAny = true
			break
		}
	}
	if hasAny {
		fmt.Println("  Day of week (days written):")
		for d := 1; d <= 7; d++ {
			fmt.Printf("    %-4s  %d\n", dowNames[d], dowCount[d])
		}
		fmt.Println()
	}
}

func cmdSearch(query string) {
	if query == "" {
		fmt.Println("  Usage: jnl search <term>")
		return
	}
	fmt.Println()
	fmt.Printf("  Searching: %q\n\n", query)
	lq := strings.ToLower(query)
	found := 0

	for _, f := range allJournalFiles(false) {
		content := readFile(f)
		if !strings.Contains(strings.ToLower(content), lq) {
			continue
		}
		fmt.Printf("  %s:\n", fileToDate(f))
		for i, line := range strings.Split(content, "\n") {
			if strings.Contains(strings.ToLower(line), lq) {
				fmt.Printf("    %d: %s\n", i+1, line)
			}
		}
		fmt.Println()
		found++
	}

	inboxContent := readFile(inboxPath)
	if strings.Contains(strings.ToLower(inboxContent), lq) {
		fmt.Println("  inbox (unfiled):")
		for i, line := range strings.Split(inboxContent, "\n") {
			if strings.Contains(strings.ToLower(line), lq) {
				fmt.Printf("    %d: %s\n", i+1, line)
			}
		}
		fmt.Println()
		found++
	}

	if found == 0 {
		fmt.Println("  Nothing found.")
	}
}

func cmdTags() {
	tagRE := regexp.MustCompile(`@[a-zA-Z][a-zA-Z0-9_-]*`)
	counts := map[string]int{}

	scan := func(path string) {
		content := readFile(path)
		for _, tag := range tagRE.FindAllString(content, -1) {
			counts[tag]++
		}
	}

	for _, f := range allJournalFiles(true) {
		scan(f)
	}
	if fileNonEmpty(inboxPath) {
		scan(inboxPath)
	}

	type tagCount struct {
		tag   string
		count int
	}
	var sorted []tagCount
	for tag, c := range counts {
		sorted = append(sorted, tagCount{tag, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count != sorted[j].count {
			return sorted[i].count > sorted[j].count
		}
		return sorted[i].tag < sorted[j].tag
	})

	fmt.Println()
	fmt.Println("  Tags used:")
	for _, tc := range sorted {
		fmt.Printf("  %-24s  %d\n", tc.tag, tc.count)
	}
	fmt.Println()
}

func cmdTag(name string) {
	if name == "" {
		fmt.Println("  Usage: jnl tag <tagname>")
		return
	}
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	fmt.Println()
	fmt.Printf("  Entries tagged %s:\n\n", name)

	for _, f := range allJournalFiles(false) {
		content := readFile(f)
		if !strings.Contains(content, name) {
			continue
		}
		fmt.Printf("  ── %s ──\n", fileToDate(f))
		for _, e := range parseEntries(content) {
			if strings.Contains(e.raw, name) {
				header := "[" + e.ts + "]"
				if e.suffix != "" {
					header += " " + e.suffix
				}
				fmt.Printf("  %s\n  %s\n\n", header, e.body)
			}
		}
	}
}

func cmdRandom() {
	files := allJournalFiles(true)
	if len(files) == 0 {
		fmt.Println("  No journal entries yet.")
		return
	}
	pick := files[rand.Intn(len(files))]
	dateStr := fileToDate(pick)
	fmt.Printf("  Random entry: %s\n", dateStr)
	cmdLog(dateStr)
}

func cmdCleanup() {
	fmt.Println()
	fmt.Println("  Scanning journal files...")

	rsqm := "\u2019" // RIGHT SINGLE QUOTATION MARK → '
	ldqm := "\u201C" // LEFT DOUBLE QUOTATION MARK  → "
	rdqm := "\u201D" // RIGHT DOUBLE QUOTATION MARK → "

	fmtChanged := 0
	ordChanged := 0
	total := 0

	for _, f := range allJournalFiles(true) {
		total++
		dateStr := fileToDate(f)
		orig := readFile(f)

		// ── formatting ────────────────────────────────────────────────────
		fixed := orig
		fixed = strings.ReplaceAll(fixed, "...", "\u2026")
		fixed = strings.ReplaceAll(fixed, rsqm, "'")
		fixed = strings.ReplaceAll(fixed, ldqm, "\"")
		fixed = strings.ReplaceAll(fixed, rdqm, "\"")
		if fixed != orig {
			writeFile(f, fixed)
			fmtChanged++
			fmt.Printf("  → %s (formatting)\n", dateStr)
			orig = fixed // use updated content for reorder check
		}

		// ── reorder entries by timestamp ──────────────────────────────────
		entries := parseEntries(orig)
		if len(entries) < 2 {
			continue
		}
		sorted := make([]entry, len(entries))
		copy(sorted, entries)
		sort.SliceStable(sorted, func(i, j int) bool {
			return sorted[i].ts < sorted[j].ts
		})
		// Check if order changed
		changed := false
		for i := range entries {
			if entries[i].ts != sorted[i].ts {
				changed = true
				break
			}
		}
		if changed {
			var parts []string
			for _, e := range sorted {
				header := "[" + e.ts + "]"
				if e.suffix != "" {
					header += " " + e.suffix
				}
				parts = append(parts, header+"\n"+e.body)
			}
			writeFile(f, strings.Join(parts, "\n\n")+"\n")
			ordChanged++
			fmt.Printf("  → %s (reordered)\n", dateStr)
		}
	}

	fmt.Println()
	if fmtChanged == 0 && ordChanged == 0 {
		fmt.Printf("  All %d file(s) already clean.\n", total)
	} else {
		if fmtChanged > 0 {
			fmt.Printf("  Formatting fixed:  %d file(s)\n", fmtChanged)
		}
		if ordChanged > 0 {
			fmt.Printf("  Entries reordered: %d file(s)\n", ordChanged)
		}
	}
	fmt.Println()
}

func cmdExport(outPath string) {
	if outPath == "" {
		outPath = filepath.Join(notesDir, "export.md")
	}
	files := allJournalFiles(true)
	if len(files) == 0 {
		fmt.Println("  No entries to export.")
		return
	}

	var sb strings.Builder
	for i, f := range files {
		content := strings.TrimRight(readFile(f), "\n")
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(content)
	}
	sb.WriteString("\n")

	if err := writeFile(outPath, sb.String()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return
	}
	totalWords := wordCount(sb.String())
	fmt.Printf("  Exported %d day(s), %d words → %s\n", len(files), totalWords, outPath)
}

func cmdOpen() {
	fmt.Printf("  Opening %s\n", journalDir)
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", journalDir).Start()
	case "windows":
		exec.Command("explorer", journalDir).Start()
	default:
		exec.Command("xdg-open", journalDir).Start()
	}
}

func cmdHelp() {
	fmt.Print(`
  jnl                    write a new draft (added to inbox)
  jnl "Title"            new draft with title pre-filled
  jnl review             work through inbox one draft at a time
  jnl browse             browse filed entries by year → month → day
  jnl inbox              view inbox contents (read-only)
  jnl log [date]         view a day's entries (default: today)
  jnl yesterday          view yesterday's entries
  jnl list               all journal files with entry + word counts
  jnl stats              streak, totals, per-year, day-of-week breakdown
  jnl search <term>      search journal + inbox
  jnl tags               all @tags with usage counts
  jnl tag <name>         all entries tagged @name
  jnl random             display a random past entry
  jnl cleanup            standardise ... → … and smart quotes; reorder timestamps
  jnl export [file]      combine all entries into one file (default: export.md)
  jnl open               open journal folder in file manager

  During review:
    ↑/↓ — navigate between drafts
    f   — file (uses draft's original date)
    e   — edit, then continue navigating
    d   — delete
    q   — quit, keep remaining

  Config (set in environment):
    JNL_DIR=~/notes   change where files live
    EDITOR=micro      terminal editor

`)
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	args := os.Args[1:]

	cmd := "new"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "new", "":
		title := ""
		if len(args) > 1 {
			title = strings.Join(args[1:], " ")
		}
		cmdNew(title)
	case "review":
		cmdReview()
	case "browse":
		cmdBrowse()
	case "inbox":
		cmdInbox()
	case "log":
		date := ""
		if len(args) > 1 {
			date = args[1]
		}
		cmdLog(date)
	case "yesterday":
		cmdYesterday()
	case "list", "ls":
		cmdList()
	case "stats":
		cmdStats()
	case "search":
		query := ""
		if len(args) > 1 {
			query = strings.Join(args[1:], " ")
		}
		cmdSearch(query)
	case "tags":
		cmdTags()
	case "tag":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		cmdTag(name)
	case "random":
		cmdRandom()
	case "cleanup":
		cmdCleanup()
	case "export":
		out := ""
		if len(args) > 1 {
			out = args[1]
		}
		cmdExport(out)
	case "open":
		cmdOpen()
	case "help", "--help", "-h":
		cmdHelp()
	default:
		// Bare date: jnl 2026-04-01
		dateRE := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
		if dateRE.MatchString(cmd) {
			cmdLog(cmd)
		} else {
			// Treat as title
			cmdNew(strings.Join(args, " "))
		}
	}
}
