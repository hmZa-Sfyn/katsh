package main

// ─────────────────────────────────────────────
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
// ─────────────────────────────────────────────

func main() {
	sh := NewShell()
	sh.Run()
}
