# jnl

A terminal journaling toolkit. Write, file, review, and search journal entries from the command line.

## Two versions

- **`jnl`** — bash script, zero dependencies, works anywhere
- **`main.go`** — Go port with identical behaviour, cross-compile with `make all`

## Install (bash)

```sh
cp jnl ~/.local/bin/jnl && chmod +x ~/.local/bin/jnl
```

## Install (Go)

```sh
go build -o ~/.local/bin/jnl .
# or cross-compile for all platforms:
make all
```

## Usage

```
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
```

## Config

Set in `~/.bashrc` or `~/.zshrc`:

```sh
export JNL_DIR=~/notes   # where files live (default: ~/notes)
export EDITOR=micro      # terminal editor (default: micro)
```

## File format

```
$JNL_DIR/
  inbox.md                    ← unsorted drafts
  journal/
    YYYY/
      MM/
        DD.md                 ← one file per day
```

Each entry is a timestamp header followed by body text:

```
[2024-03-15 09:30:00] Optional title
Body of the entry here.
```
