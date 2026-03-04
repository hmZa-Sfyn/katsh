package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Shell — runtime state
// ─────────────────────────────────────────────────────────────────────────────

type HistoryEntry struct {
	Raw      string    `json:"raw"`
	At       time.Time `json:"at"`
	ExitCode int       `json:"exit_code"`
}

type Shell struct {
	cwd      string
	prevDir  string
	dirStack []string
	box      *Box
	history  []HistoryEntry
	aliases  map[string]Alias
	vars     map[string]string
	funcs    map[string]*UserFunc
	lastCode int

	// Output capture (for backtick subshells)
	captureMode bool
	captureOut  bytes.Buffer
}

func NewShell() *Shell {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}
	sh := &Shell{
		cwd:     cwd,
		box:     NewBox(),
		aliases: make(map[string]Alias),
		vars:    make(map[string]string),
		funcs:   make(map[string]*UserFunc),
	}
	sh.loadHistory()
	return sh
}

// ─────────────────────────────────────────────────────────────────────────────
//  REPL
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) Run() {
	fmt.Print(banner())

	for {
		prompt := renderPrompt(sh.cwd, os.Getenv("USER"), sh.lastCode)
		line, eof := sh.Readline(prompt)
		if eof {
			break
		}

		raw := strings.TrimSpace(line)
		if raw == "" {
			continue
		}

		// Add to in-memory history
		sh.history = append(sh.history, HistoryEntry{Raw: raw, At: time.Now()})

		code := sh.execLine(raw)
		sh.lastCode = code
		sh.history[len(sh.history)-1].ExitCode = code

		// Persist to disk after every command
		sh.saveHistory()
	}

	fmt.Println(c(ansiGrey, "\nbye."))
}

// ─────────────────────────────────────────────────────────────────────────────
//  execLine — parse and execute one raw input line
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) execLine(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	// ── Expand backticks in the raw line before anything else ─────────────
	raw = sh.expandBackticks(raw)

	// ── Try scripting engine first (if/for/while/func/variables) ──────────
	if handled, code := sh.evalScript(raw); handled {
		return code
	}

	// ── Standard command path ─────────────────────────────────────────────
	raw = sh.expandAliases(raw)
	raw = sh.expandVars(raw)

	pc := Parse(raw)
	if len(pc.Args) == 0 {
		return 0
	}

	command := pc.Args[0]
	args := pc.Args[1:]

	// ── Built-ins ─────────────────────────────────────────────────────────
	result, wasBuiltin, err := handleBuiltin(sh, command, args)

	if err == errExit {
		sh.saveHistory()
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
			sh.printErr(wrapErr(err, raw))
			return 1
		}
		if result != nil && (result.IsTable || strings.TrimSpace(result.Text) != "") {
			result, err = ApplyPipes(result, pc.Pipes)
			if err != nil {
				// Check if error is already a ShellError (don't double-wrap)
				if se, ok := err.(*ShellError); ok {
					se.Source = raw // Update source to show full command
					sh.printErr(se)
				} else {
					sh.printErr(wrapErr(err, raw))
				}
				return 1
			}
			sh.printResult(result)
		}
		if pc.ShouldStore && result != nil {
			sh.storeResult(pc, command, result)
		}
		return 0
	}

	// ── Unknown command — check for typo ──────────────────────────────────
	if findInPath(command) == "" {
		sh.printErr(errUnknownCmd(command, raw))
		return 127
	}

	// ── External command ──────────────────────────────────────────────────
	result, err = RunExternal(command, args, sh.cwd)
	if err != nil {
		sh.printErr(wrapErr(err, raw))
		return 1
	}

	result, err = ApplyPipes(result, pc.Pipes)
	if err != nil {
		// Check if error is already a ShellError (don't double-wrap)
		if se, ok := err.(*ShellError); ok {
			se.Source = raw // Update source to show full command
			sh.printErr(se)
		} else {
			sh.printErr(wrapErr(err, raw))
		}
		return 1
	}

	sh.printResult(result)

	if pc.ShouldStore {
		sh.storeResult(pc, command, result)
	}

	return 0
}

// ─────────────────────────────────────────────────────────────────────────────
//  Output rendering  (respects capture mode for backtick subshells)
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) printResult(r *Result) {
	if r == nil {
		return
	}
	if r.IsTable {
		lines := RenderTable(r.Cols, r.Rows)
		if sh.captureMode {
			// Return rows as newline-separated column values
			for _, row := range r.Rows {
				var parts []string
				for _, col := range r.Cols {
					parts = append(parts, row[col])
				}
				sh.captureOut.WriteString(strings.Join(parts, "\t") + "\n")
			}
			return
		}
		fmt.Println()
		for _, line := range lines {
			fmt.Println(line)
		}
		fmt.Println()
	} else {
		text := strings.TrimRight(r.Text, "\n")
		if text == "" {
			return
		}
		if sh.captureMode {
			sh.captureOut.WriteString(text)
			return
		}
		fmt.Println()
		for _, line := range strings.Split(text, "\n") {
			fmt.Println("  " + line)
		}
		fmt.Println()
	}
}

func (sh *Shell) printErr(e *ShellError) {
	if e == nil {
		return
	}
	if sh.captureMode {
		// In capture mode just write to stderr
		fmt.Fprintln(os.Stderr, e.Message)
		return
	}
	PrintError(e)
}

// ─────────────────────────────────────────────────────────────────────────────
//  Box store helpers
// ─────────────────────────────────────────────────────────────────────────────

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
		ansiBold, ansiCyan, e.Key, ansiReset,
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

// ─────────────────────────────────────────────────────────────────────────────
//  Alias & variable expansion
// ─────────────────────────────────────────────────────────────────────────────

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

func (sh *Shell) expandVars(raw string) string {
	return os.Expand(raw, func(key string) string {
		if v, ok := sh.vars[key]; ok {
			return v
		}
		return os.Getenv(key)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
//  History persistence
// ─────────────────────────────────────────────────────────────────────────────

// historyPaths returns (tempPath, permanentPath) for the current OS.
func historyPaths() (string, string) {
	var tmpDir, permDir string

	if runtime.GOOS == "windows" {
		tmpDir = filepath.Join(os.Getenv("TEMP"), "structsh")
		permDir = filepath.Join(os.Getenv("APPDATA"), "structsh")
	} else {
		tmpDir = filepath.Join("/tmp", "structsh")
		home := os.Getenv("HOME")
		if home == "" {
			home = "/"
		}
		permDir = filepath.Join(home, ".config", "structsh")
	}

	_ = os.MkdirAll(tmpDir, 0755)
	_ = os.MkdirAll(permDir, 0755)

	return filepath.Join(tmpDir, "history.json"),
		filepath.Join(permDir, "history.json")
}

func (sh *Shell) loadHistory() {
	_, permPath := historyPaths()
	data, err := os.ReadFile(permPath)
	if err != nil {
		return // no history yet — that's fine
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	sh.history = entries
}

func (sh *Shell) saveHistory() {
	tmpPath, permPath := historyPaths()

	// Keep last 10000 entries
	entries := sh.history
	if len(entries) > 10000 {
		entries = entries[len(entries)-10000:]
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(tmpPath, data, 0600)
	_ = os.WriteFile(permPath, data, 0600)
}
