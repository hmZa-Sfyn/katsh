package main

import (
	"os"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
//  StructSH — Structured Shell
//
//  File layout (all package main):
//    main.go       — entry point
//    types.go      — shared types: Row, Result, BoxEntry, ParsedCommand, Alias
//    ansi.go       — terminal colors and prompt rendering
//    table.go      — aligned table renderer with semantic coloring
//    box.go        — in-memory session store (Box)
//    parser.go     — command line parser (pipes, #= operator, quotes)
//    executor.go   — runs OS commands, parses output into Results
//    pipes.go      — pipe transforms: select/where/sort/limit/grep/fmt/...
//    builtins.go   — built-in commands: cd/cat/find/stat/alias/box/help/...
//    shell.go      — Shell struct, REPL loop, rendering, alias expansion
// ─────────────────────────────────────────────────────────────────────────────

func main() {
	sh := NewShell()

	// Check for -c flag to run a single command
	if len(os.Args) >= 2 && os.Args[1] == "-c" && len(os.Args) > 2 {
		cmd := strings.Join(os.Args[2:], " ")
		code := sh.execLine(cmd)
		sh.saveHistory()
		if code != 0 {
			os.Exit(code)
		}
		return
	}

	// Otherwise run the interactive shell
	sh.Run()
}
