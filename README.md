# StructSH — Structured Shell

> Everything is data. Every output is a table.

A Go shell where all command output is **structured** — parsed into typed rows and columns. Filter, transform, sort, and store results with a clean pipe syntax. All files are `package main` with zero external dependencies.

---

## Quick Start

```sh
git clone <repo>
cd structsh
go run .

# or build:
go build -o structsh .
./structsh
```

**Requires:** Go 1.22+, no external dependencies.

---

## File Layout

| File | Responsibility |
|---|---|
| `main.go` | Entry point only |
| `types.go` | Shared types: `Row`, `Result`, `BoxEntry`, `ParsedCommand`, `Alias` |
| `ansi.go` | ANSI color codes, prompt rendering |
| `table.go` | Aligned, color-coded table renderer |
| `box.go` | In-memory session store (Box) with tags, export/import |
| `parser.go` | Command line parser: pipes, `#=`, quotes, comments |
| `executor.go` | Runs OS commands, auto-parses output into tables |
| `pipes.go` | Pipe transforms: `select`, `where`, `sort`, `fmt`, etc. |
| `builtins.go` | Built-in commands: `cd`, `cat`, `find`, `alias`, `box`, etc. |
| `shell.go` | `Shell` struct, REPL loop, alias expansion, variable expansion |

---

## Syntax

### Basic command
```sh
ls -la
ps aux
env
df -h
```

### Pipe transforms (chainable)
```sh
ls -la | select name, size
ps aux | where cpu>5 | sort cpu desc | limit 10
env | grep PATH
ls | where type=dir | count
cat app.log | grep error | limit 20
env | fmt json
ps | select pid,command | fmt csv
```

### Where operators
| Operator | Meaning |
|---|---|
| `=` | exact match (case-insensitive) |
| `!=` | not equal |
| `>` `<` `>=` `<=` | numeric comparison |
| `~` | contains (substring) |

### Box storage
```sh
ls -la #=               # auto-name (out_1, out_2, ...)
ls -la #=myfiles        # named store

box                     # list all
box get myfiles         # retrieve by name
box get 3               # retrieve by id
box rm myfiles          # remove
box rename myfiles dirs # rename
box tag myfiles work    # tag
box filter tag work     # list by tag
box search go           # search name/source
box export snap.json    # export to JSON
box import snap.json    # import from JSON
box clear               # wipe all
```

### All pipe operators
| Pipe | Description |
|---|---|
| `\| select a,b,c` | keep only these columns |
| `\| where col=val` | filter rows |
| `\| grep text` | search all columns/lines |
| `\| sort col [asc\|desc]` | sort rows |
| `\| limit N` | keep first N rows |
| `\| skip N` | skip first N rows |
| `\| count` | count rows |
| `\| unique [col]` | deduplicate |
| `\| reverse` | flip row order |
| `\| fmt json\|csv\|tsv` | reformat output |
| `\| add col=value` | add a column |
| `\| rename old=new` | rename a column |

### Session variables
```sh
set MYVAR=hello
echo $MYVAR
unset MYVAR
vars           # show all session vars
```

### Aliases
```sh
alias ll=ls -la
alias lsd=ls -la | where type=dir
aliases        # list all
unalias ll
```

### Directory stack
```sh
pushd /tmp
popd
cd -           # go back to previous dir
```

### Other built-ins
```sh
cat file.txt
head -n 5 file.txt
tail -n 20 file.txt
wc file.txt            # lines/words/bytes as table
stat file.txt          # file metadata as table
which go               # find binary in PATH as table
find . -name *.go -type f
mkdir -p a/b/c
touch newfile.txt
cp src dst
mv old new
rm -rf dir/
history 20             # last 20 commands as table
```

---

## Example session

```
~ ❯ ls -la | where type=dir | select name, size
  NAME         SIZE
  ───────────  ──────
  projects/    4.0K
  .config/     4.0K
  2 row(s)

~ ❯ ps aux | where cpu>1 | sort cpu desc | limit 5 #=hotprocs
  📦 box["hotprocs"] id:1  5 rows

~ ❯ box get hotprocs
  ◈ box["hotprocs"] (id:1)  14:02:31
  $ ps aux

  PID    USER   CPU     MEM    COMMAND
  ─────  ─────  ──────  ─────  ──────────────────
  1102   user   45.3%   12.1%  go build ./...
  ...

~ ❯ env | grep GOPATH | fmt json
  [
    {"key": "GOPATH", "value": "/home/user/go"}
  ]
```
