# jnl

A terminal journaling toolkit. Write, file, review, and search journal entries from the command line.

## Install

**Download a pre-built binary** (no Go required):

| Platform | Binary |
|----------|--------|
| macOS — Apple Silicon | `jnl-macos-arm64` |
| macOS — Intel | `jnl-macos-amd64` |
| Linux — x86-64 | `jnl-linux-amd64` |
| Linux — arm64 (Raspberry Pi 4/5, Zero 2W) | `jnl-linux-arm64` |
| Linux — ARMv6 (Raspberry Pi Zero) | `jnl-linux-armv6` |

```sh
# macOS Apple Silicon
curl -L https://github.com/Caddickbrown/jnl/releases/latest/download/jnl-macos-arm64 \
  -o ~/.local/bin/jnl && chmod +x ~/.local/bin/jnl

# Linux x86-64
curl -L https://github.com/Caddickbrown/jnl/releases/latest/download/jnl-linux-amd64 \
  -o ~/.local/bin/jnl && chmod +x ~/.local/bin/jnl

# Raspberry Pi Zero
curl -L https://github.com/Caddickbrown/jnl/releases/latest/download/jnl-linux-armv6 \
  -o ~/.local/bin/jnl && chmod +x ~/.local/bin/jnl
```

Make sure `~/.local/bin` is on your PATH (`echo 'export PATH=$PATH:~/.local/bin' >> ~/.bashrc`).

**Build from source** (requires Go):

```sh
make install    # builds for current platform → ~/.local/bin/jnl
make all        # cross-compiles all platforms into ./dist/
```

## Tab autocomplete

**bash** — add to `~/.bashrc`:

```bash
_jnl_complete() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        local cmds="review browse inbox sort log yesterday list stats search tags tag random cleanup export open config help"
        COMPREPLY=($(compgen -W "$cmds" -- "$cur"))
    fi
}
complete -F _jnl_complete jnl
```

**zsh** — add to `~/.zshrc`:

```zsh
_jnl() {
    local -a cmds
    cmds=(
        'review:work through inbox one draft at a time'
        'browse:browse filed entries by year → month → day'
        'inbox:view inbox contents'
        'sort:sort inbox.md entries into date order'
        'log:view a day''s entries'
        'yesterday:view yesterday''s entries'
        'list:all journal files with word counts'
        'stats:streak, totals, day-of-week breakdown'
        'search:search journal + inbox'
        'tags:all @tags with usage counts'
        'tag:entries for a specific @tag'
        'random:display a random past entry'
        'cleanup:standardise punctuation and reorder timestamps'
        'export:combine all entries into one file'
        'open:open journal folder in file manager'
        'config:open config file in editor'
    )
    _describe 'jnl command' cmds
}
compdef _jnl jnl
```

## Usage

```
jnl                    write a new draft (added to inbox)
jnl "Title"            new draft with title pre-filled
jnl review             work through inbox one draft at a time
jnl browse             browse filed entries by year → month → day
jnl inbox              view inbox contents (read-only)
jnl sort               sort inbox.md entries into date order (in place)
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
jnl config             interactively change configuration
```

## Tags

Tags are `@word` tokens written anywhere in an entry body. A tag starts with `@` followed by a letter, then any mix of letters, numbers, underscores, or hyphens — for example `@work`, `@ideas`, `@health-check`.

```
[2024-03-15 09:30:00] Morning thoughts
Feeling good today. Need to follow up on the @work project and book a @health appointment.
```

**Listing tags**

```sh
jnl tags          # all @tags across the journal, sorted by usage count
```

**Filtering by tag**

```sh
jnl tag work      # all entries containing @work (@ prefix optional)
jnl tag @work     # same
```

**Routing entries to their own file**

Set `JNL_SPLIT_TAGS` to a space- or comma-separated list of tags. When you file a draft that contains one of those tags, it is written to `$JNL_DIR/<tagname>.md` instead of the normal date-based journal file.

```sh
export JNL_SPLIT_TAGS="work private"
# @work entries    → ~/notes/work.md
# @private entries → ~/notes/private.md
```

## Config

Run `jnl config` for an interactive wizard, or set env vars in `~/.bashrc` / `~/.zshrc`:

```sh
export JNL_DIR=~/notes               # where files live (default: ~/notes)
export EDITOR=micro                  # use micro as the editor
export EDITOR=builtin                # always use the built-in editor (no external tool needed)
export JNL_SPLIT_TAGS="work private" # tags that route to their own file
```

Settings are saved to `~/.config/jnl/config`. Env vars always override the config file.

**Editor priority:**
1. `$EDITOR` env var — used unconditionally if set (`builtin` or `default` forces the built-in)
2. `micro` — used if found on PATH
3. Built-in editor — used if nothing else is available (always works, no install needed)

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
