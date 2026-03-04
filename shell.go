package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ─────────────────────────────────────────────
//  Shell — the main REPL struct
// ─────────────────────────────────────────────

// HistoryEntry is one recorded command.
type HistoryEntry struct {
	Raw      string
	At       time.Time
	ExitCode int
}

// Shell holds all runtime state.
type Shell struct {
	cwd      string
	prevDir  string
	dirStack []string
	box      *Box
	history  []HistoryEntry
	aliases  map[string]Alias
	vars     map[string]string // shell session variables
	lastCode int               // exit code of last command
}

// NewShell creates an initialized Shell.
func NewShell() *Shell {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}
	return &Shell{
		cwd:     cwd,
		box:     NewBox(),
		aliases: make(map[string]Alias),
		vars:    make(map[string]string),
	}
}

// ─────────────────────────────────────────────
//  REPL
// ─────────────────────────────────────────────

func (sh *Shell) Run() {
	fmt.Print(banner())
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(renderPrompt(sh.cwd, os.Getenv("USER"), sh.lastCode))

		if !scanner.Scan() {
			break // EOF / Ctrl-D
		}

		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}

		sh.history = append(sh.history, HistoryEntry{Raw: raw, At: time.Now()})

		code := sh.execLine(raw)
		sh.lastCode = code
		sh.history[len(sh.history)-1].ExitCode = code
	}

	fmt.Println(c(ansiGrey, "\nbye."))
}

// execLine runs one raw line, returns exit code (0 = ok).
func (sh *Shell) execLine(raw string) int {
	// Expand aliases first
	raw = sh.expandAliases(raw)

	// Expand shell variables ($NAME)
	raw = sh.expandVars(raw)

	pc := Parse(raw)
	if len(pc.Args) == 0 {
		return 0
	}

	command := pc.Args[0]
	args := pc.Args[1:]

	// ── Built-ins ──────────────────────────────
	result, wasBuiltin, err := handleBuiltin(sh, command, args)

	if err == errExit {
		fmt.Println(c(ansiGrey, "bye."))
		os.Exit(0)
	}
	if err == errClear {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
		return 0
	}

	if wasBuiltin {
		if err != nil {
			fmt.Println(errMsg(err.Error()))
			return 1
		}
		if result != nil && (result.IsTable || strings.TrimSpace(result.Text) != "") {
			// Apply any pipe stages even on built-in results
			result, err = ApplyPipes(result, pc.Pipes)
			if err != nil {
				fmt.Println(errMsg(err.Error()))
				return 1
			}
			sh.printResult(result)
		}
		if pc.ShouldStore && result != nil {
			sh.storeResult(pc, command, result)
		}
		return 0
	}

	// ── External command ───────────────────────
	result, err = RunExternal(command, args, sh.cwd)
	if err != nil {
		fmt.Println(errMsg(err.Error()))
		return 1
	}

	// ── Apply pipes ────────────────────────────
	result, err = ApplyPipes(result, pc.Pipes)
	if err != nil {
		fmt.Println(errMsg(err.Error()))
		return 1
	}

	// ── Print output ───────────────────────────
	sh.printResult(result)

	// ── Store in box ───────────────────────────
	if pc.ShouldStore {
		sh.storeResult(pc, command, result)
	}

	return 0
}

// ─────────────────────────────────────────────
//  Output rendering
// ─────────────────────────────────────────────

func (sh *Shell) printResult(r *Result) {
	if r == nil {
		return
	}
	fmt.Println()
	if r.IsTable {
		for _, line := range RenderTable(r.Cols, r.Rows) {
			fmt.Println(line)
		}
	} else {
		text := strings.TrimRight(r.Text, "\n")
		if text == "" {
			return
		}
		for _, line := range strings.Split(text, "\n") {
			fmt.Println("  " + line)
		}
	}
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Box store helpers
// ─────────────────────────────────────────────

func (sh *Shell) storeResult(pc *ParsedCommand, command string, r *Result) {
	key := pc.StoreKey
	src := strings.Join(pc.Args, " ")
	if key == "" {
		key = sh.box.autoKey()
	}

	var e *BoxEntry
	if r.IsTable {
		e = sh.box.StoreTable(key, src, r.Cols, r.Rows)
	} else {
		e = sh.box.StoreText(key, src, r.Text)
	}

	fmt.Println(boxMsg(e.Key, e.ID, e.Size()))
	fmt.Println()
}

func (sh *Shell) printBoxList(entries []*BoxEntry) {
	fmt.Println()
	fmt.Printf("  %s%s◈ BOX  (%d entries)%s\n\n", ansiBold, ansiCyan, len(entries), ansiReset)
	cols := []string{"id", "key", "type", "size", "tags", "created", "source"}
	var rows []Row
	for _, e := range entries {
		tags := strings.Join(e.Tags, " #")
		if tags != "" {
			tags = "#" + tags
		}
		rows = append(rows, Row{
			"id":      fmt.Sprintf("%d", e.ID),
			"key":     e.Key,
			"type":    string(e.Type),
			"size":    e.Size(),
			"tags":    tags,
			"created": e.Created.Format("15:04:05"),
			"source":  truncStr(e.Source, 36),
		})
	}
	for _, line := range RenderTable(cols, rows) {
		fmt.Println(line)
	}
	fmt.Println()
	fmt.Println(c(ansiGrey, "  box get <key>  ·  box rm <key>  ·  box rename old new  ·  box tag <key> <tag>  ·  box export <file>"))
	fmt.Println()
}

func (sh *Shell) printBoxEntry(e *BoxEntry) {
	tags := ""
	if len(e.Tags) > 0 {
		tags = "  " + c(ansiMagenta, "#"+strings.Join(e.Tags, " #"))
	}
	fmt.Printf("\n  %s%s◈ box[%q]%s  %sid:%d%s%s%s\n\n",
		ansiBold, ansiCyan,
		e.Key, ansiReset,
		ansiGrey, e.ID, ansiReset,
		tags,
		c(ansiGrey, "  "+e.Created.Format("15:04:05")),
	)
	if e.Source != "" {
		fmt.Printf("  %s$ %s%s\n\n", ansiGrey, e.Source, ansiReset)
	}
	switch e.Type {
	case TypeTable:
		for _, line := range RenderTable(e.Cols, e.Rows) {
			fmt.Println(line)
		}
	case TypeText:
		for _, line := range strings.Split(strings.TrimRight(e.Text, "\n"), "\n") {
			fmt.Println("  " + line)
		}
	}
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Alias & variable expansion
// ─────────────────────────────────────────────

// expandAliases replaces the first token of a command with its alias expansion.
func (sh *Shell) expandAliases(raw string) string {
	parts := strings.SplitN(raw, " ", 2)
	name := parts[0]
	if a, ok := sh.aliases[name]; ok {
		if len(parts) > 1 {
			return a.Expand + " " + parts[1]
		}
		return a.Expand
	}
	return raw
}

// expandVars replaces $VAR tokens with their value from sh.vars or os.Environ.
func (sh *Shell) expandVars(raw string) string {
	return os.Expand(raw, func(key string) string {
		if v, ok := sh.vars[key]; ok {
			return v
		}
		return os.Getenv(key)
	})
}
