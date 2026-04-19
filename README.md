# jnl

A terminal journaling toolkit. Write, file, review, and search journal entries from the command line.

## Install

```sh
make install
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
jnl extract <tag> [file]
                       copy or cut all entries tagged @tag to a file
                       prompts for copy/cut  ·  default file: ~/notes/<tag>.md
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
# @work entries  → ~/notes/work.md
# @private entries → ~/notes/private.md
```

This is useful for keeping specific topics (a work log, a private diary) in their own file while the rest of your journal stays in the date hierarchy.

## Config

Run `jnl config` for an interactive wizard, or set env vars in `~/.bashrc` / `~/.zshrc`:

```sh
export JNL_DIR=~/notes               # where files live (default: ~/notes)
export EDITOR=micro                  # terminal editor (default: micro)
export JNL_SPLIT_TAGS="work private" # tags that route to their own file
                                     # @work entries → ~/notes/work.md
```

Settings are saved to `~/.config/jnl/config`. Env vars always override the config file.

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
