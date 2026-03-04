package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─────────────────────────────────────────────
//  Built-in shell commands
//  (commands that must run in-process)
// ─────────────────────────────────────────────

// handleBuiltin checks if cmd is a built-in and runs it.
// Returns (result, wasBuiltin, error).
func handleBuiltin(sh *Shell, command string, args []string) (*Result, bool, error) {
	switch command {

	// ── Navigation ──────────────────────────────
	case "cd":
		return builtinCD(sh, args)
	case "pwd":
		return NewText(sh.cwd), true, nil
	case "pushd":
		return builtinPushd(sh, args)
	case "popd":
		return builtinPopd(sh)

	// ── File operations ─────────────────────────
	case "cat":
		return builtinCat(sh, args)
	case "head":
		return builtinHead(sh, args)
	case "tail":
		return builtinTail(sh, args)
	case "wc":
		return builtinWC(sh, args)
	case "touch":
		return builtinTouch(sh, args)
	case "mkdir":
		return builtinMkdir(sh, args)
	case "rm":
		return builtinRm(sh, args)
	case "cp":
		return builtinCp(sh, args)
	case "mv":
		return builtinMv(sh, args)
	case "stat":
		return builtinStat(sh, args)
	case "which":
		return builtinWhich(sh, args)
	case "find":
		return builtinFind(sh, args)

	// ── Shell state ─────────────────────────────
	case "echo":
		return NewText(strings.Join(args, " ")), true, nil
	case "set":
		return builtinSet(sh, args)
	case "unset":
		return builtinUnset(sh, args)
	case "vars":
		return builtinVars(sh)
	case "alias":
		return builtinAlias(sh, args)
	case "unalias":
		return builtinUnalias(sh, args)
	case "aliases":
		return builtinListAliases(sh)

	// ── Box ─────────────────────────────────────
	case "box":
		return builtinBox(sh, args)

	// ── Session ─────────────────────────────────
	case "history":
		return builtinHistory(sh, args)
	case "clear":
		return nil, true, errClear
	case "help":
		return builtinHelp()
	case "exit", "quit":
		return nil, true, errExit
	}

	return nil, false, nil
}

// sentinel errors
var errExit = fmt.Errorf("__exit__")
var errClear = fmt.Errorf("__clear__")

// ── cd ────────────────────────────────────────

func builtinCD(sh *Shell, args []string) (*Result, bool, error) {
	target := homeDir()
	if len(args) > 0 {
		target = args[0]
	}
	// Handle ~ expansion
	if target == "~" || target == "" {
		target = homeDir()
	} else if strings.HasPrefix(target, "~/") {
		target = filepath.Join(homeDir(), target[2:])
	} else if target == "-" {
		// cd - goes to previous dir
		if sh.prevDir != "" {
			target = sh.prevDir
		} else {
			return nil, true, fmt.Errorf("cd: no previous directory")
		}
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(sh.cwd, target)
	}
	target = filepath.Clean(target)
	if err := os.Chdir(target); err != nil {
		return nil, true, fmt.Errorf("cd: %s: %w", args[0], err)
	}
	sh.prevDir = sh.cwd
	sh.cwd = target
	return NewText(""), true, nil
}

// ── pushd / popd ─────────────────────────────

func builtinPushd(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("pushd: requires a directory argument")
	}
	sh.dirStack = append(sh.dirStack, sh.cwd)
	_, _, err := builtinCD(sh, args)
	if err != nil {
		sh.dirStack = sh.dirStack[:len(sh.dirStack)-1]
		return nil, true, err
	}
	// Show the stack
	var lines []string
	for i := len(sh.dirStack) - 1; i >= 0; i-- {
		lines = append(lines, fmt.Sprintf("  %d  %s", i, sh.dirStack[i]))
	}
	lines = append(lines, fmt.Sprintf("  →  %s (current)", sh.cwd))
	return NewText(strings.Join(lines, "\n")), true, nil
}

func builtinPopd(sh *Shell) (*Result, bool, error) {
	if len(sh.dirStack) == 0 {
		return nil, true, fmt.Errorf("popd: directory stack empty")
	}
	n := len(sh.dirStack)
	prev := sh.dirStack[n-1]
	sh.dirStack = sh.dirStack[:n-1]
	_, _, err := builtinCD(sh, []string{prev})
	return NewText(sh.cwd), true, err
}

// ── cat / head / tail ─────────────────────────

func builtinCat(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("cat: missing file argument")
	}
	var parts []string
	for _, arg := range args {
		p := resolvePath(sh.cwd, arg)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("cat: %s: %w", arg, err)
		}
		parts = append(parts, string(data))
	}
	return NewText(strings.Join(parts, "")), true, nil
}

func builtinHead(sh *Shell, args []string) (*Result, bool, error) {
	n := 10
	var files []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &n)
			i++
		} else {
			files = append(files, args[i])
		}
	}
	if len(files) == 0 {
		return nil, true, fmt.Errorf("head: missing file argument")
	}
	var out []string
	for _, f := range files {
		data, err := os.ReadFile(resolvePath(sh.cwd, f))
		if err != nil {
			return nil, true, fmt.Errorf("head: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		if n < len(lines) {
			lines = lines[:n]
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	return NewText(strings.Join(out, "\n")), true, nil
}

func builtinTail(sh *Shell, args []string) (*Result, bool, error) {
	n := 10
	var files []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &n)
			i++
		} else {
			files = append(files, args[i])
		}
	}
	if len(files) == 0 {
		return nil, true, fmt.Errorf("tail: missing file argument")
	}
	var out []string
	for _, f := range files {
		data, err := os.ReadFile(resolvePath(sh.cwd, f))
		if err != nil {
			return nil, true, fmt.Errorf("tail: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		if n < len(lines) {
			lines = lines[len(lines)-n:]
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	return NewText(strings.Join(out, "\n")), true, nil
}

// ── wc ────────────────────────────────────────

func builtinWC(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("wc: missing file argument")
	}
	cols := []string{"lines", "words", "bytes", "file"}
	var rows []Row
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("wc: %s: %w", f, err)
		}
		lines := len(strings.Split(string(data), "\n")) - 1
		words := len(strings.Fields(string(data)))
		rows = append(rows, Row{
			"lines": fmt.Sprintf("%d", lines),
			"words": fmt.Sprintf("%d", words),
			"bytes": fmt.Sprintf("%d", len(data)),
			"file":  f,
		})
	}
	return NewTable(cols, rows), true, nil
}

// ── touch ─────────────────────────────────────

func builtinTouch(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("touch: missing file argument")
	}
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, []byte{}, 0644); err != nil {
				return nil, true, fmt.Errorf("touch: %s: %w", f, err)
			}
		} else {
			now := time.Now()
			if err := os.Chtimes(p, now, now); err != nil {
				return nil, true, fmt.Errorf("touch: %s: %w", f, err)
			}
		}
	}
	return NewText(""), true, nil
}

// ── mkdir ─────────────────────────────────────

func builtinMkdir(sh *Shell, args []string) (*Result, bool, error) {
	parents := false
	var dirs []string
	for _, a := range args {
		if a == "-p" {
			parents = true
		} else {
			dirs = append(dirs, a)
		}
	}
	if len(dirs) == 0 {
		return nil, true, fmt.Errorf("mkdir: missing operand")
	}
	for _, d := range dirs {
		p := resolvePath(sh.cwd, d)
		var err error
		if parents {
			err = os.MkdirAll(p, 0755)
		} else {
			err = os.Mkdir(p, 0755)
		}
		if err != nil {
			return nil, true, fmt.Errorf("mkdir: %s: %w", d, err)
		}
	}
	return NewText(""), true, nil
}

// ── rm ────────────────────────────────────────

func builtinRm(sh *Shell, args []string) (*Result, bool, error) {
	recursive := false
	force := false
	var targets []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "r") || strings.Contains(a, "R") {
				recursive = true
			}
			if strings.Contains(a, "f") {
				force = true
			}
		} else {
			targets = append(targets, a)
		}
	}
	if len(targets) == 0 {
		return nil, true, fmt.Errorf("rm: missing operand")
	}
	for _, t := range targets {
		p := resolvePath(sh.cwd, t)
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			if force {
				continue
			}
			return nil, true, fmt.Errorf("rm: %s: no such file or directory", t)
		}
		if info.IsDir() && !recursive {
			return nil, true, fmt.Errorf("rm: %s: is a directory (use -r)", t)
		}
		if recursive {
			err = os.RemoveAll(p)
		} else {
			err = os.Remove(p)
		}
		if err != nil {
			return nil, true, fmt.Errorf("rm: %s: %w", t, err)
		}
	}
	return NewText(""), true, nil
}

// ── cp / mv ───────────────────────────────────

func builtinCp(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 2 {
		return nil, true, fmt.Errorf("cp: usage: cp <src> <dst>")
	}
	src := resolvePath(sh.cwd, args[0])
	dst := resolvePath(sh.cwd, args[1])
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, true, fmt.Errorf("cp: %w", err)
	}
	info, _ := os.Stat(src)
	mode := os.FileMode(0644)
	if info != nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		return nil, true, fmt.Errorf("cp: %w", err)
	}
	return NewText(""), true, nil
}

func builtinMv(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 2 {
		return nil, true, fmt.Errorf("mv: usage: mv <src> <dst>")
	}
	src := resolvePath(sh.cwd, args[0])
	dst := resolvePath(sh.cwd, args[1])
	if err := os.Rename(src, dst); err != nil {
		return nil, true, fmt.Errorf("mv: %w", err)
	}
	return NewText(""), true, nil
}

// ── stat ──────────────────────────────────────

func builtinStat(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("stat: missing file argument")
	}
	cols := []string{"name", "size", "mode", "modified", "is_dir"}
	var rows []Row
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		info, err := os.Stat(p)
		if err != nil {
			return nil, true, fmt.Errorf("stat: %s: %w", f, err)
		}
		isDir := "no"
		if info.IsDir() {
			isDir = "yes"
		}
		rows = append(rows, Row{
			"name":     f,
			"size":     fmtBytes(info.Size()),
			"mode":     info.Mode().String(),
			"modified": info.ModTime().Format("2006-01-02 15:04:05"),
			"is_dir":   isDir,
		})
	}
	return NewTable(cols, rows), true, nil
}

// ── which ─────────────────────────────────────

func builtinWhich(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("which: missing argument")
	}
	cols := []string{"command", "path"}
	var rows []Row
	for _, cmd := range args {
		p := findInPath(cmd)
		if p == "" {
			p = "(not found)"
		}
		rows = append(rows, Row{"command": cmd, "path": p})
	}
	return NewTable(cols, rows), true, nil
}

// ── find ──────────────────────────────────────
// find [dir] [-name pattern] [-type f|d] [-size +N]

func builtinFind(sh *Shell, args []string) (*Result, bool, error) {
	dir := sh.cwd
	namePattern := ""
	typeFilter := ""
	var maxDepth int = 20

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-name":
			if i+1 < len(args) {
				namePattern = args[i+1]
				i++
			}
		case "-type":
			if i+1 < len(args) {
				typeFilter = args[i+1]
				i++
			}
		case "-maxdepth":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &maxDepth)
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				dir = resolvePath(sh.cwd, args[i])
			}
		}
	}

	cols := []string{"path", "type", "size", "modified"}
	var rows []Row
	_ = walkFind(dir, dir, namePattern, typeFilter, maxDepth, 0, &rows)
	if len(rows) == 0 {
		return NewText("(no matches)"), true, nil
	}
	return NewTable(cols, rows), true, nil
}

func walkFind(root, dir, namePattern, typeFilter string, maxDepth, depth int, rows *[]Row) error {
	if depth > maxDepth {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())
		relPath, _ := filepath.Rel(root, fullPath)
		t := "file"
		if e.IsDir() {
			t = "dir"
		}
		// Apply filters
		if typeFilter != "" {
			if typeFilter == "f" && t != "file" {
				goto descend
			}
			if typeFilter == "d" && t != "dir" {
				goto descend
			}
		}
		if namePattern != "" {
			matched, _ := filepath.Match(namePattern, e.Name())
			if !matched {
				goto descend
			}
		}
		{
			info, _ := e.Info()
			size := ""
			mod := ""
			if info != nil {
				size = fmtBytes(info.Size())
				mod = info.ModTime().Format("2006-01-02 15:04")
			}
			*rows = append(*rows, Row{
				"path":     "./" + relPath,
				"type":     t,
				"size":     size,
				"modified": mod,
			})
		}
	descend:
		if e.IsDir() {
			_ = walkFind(root, fullPath, namePattern, typeFilter, maxDepth, depth+1, rows)
		}
	}
	return nil
}

// ── set / unset / vars ────────────────────────

func builtinSet(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return builtinVars(sh)
	}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			return nil, true, fmt.Errorf("set: invalid format (use NAME=VALUE)")
		}
		sh.vars[parts[0]] = parts[1]
	}
	return NewText(""), true, nil
}

func builtinUnset(sh *Shell, args []string) (*Result, bool, error) {
	for _, a := range args {
		delete(sh.vars, a)
	}
	return NewText(""), true, nil
}

func builtinVars(sh *Shell) (*Result, bool, error) {
	cols := []string{"name", "value"}
	var rows []Row
	for k, v := range sh.vars {
		rows = append(rows, Row{"name": k, "value": v})
	}
	return NewTable(cols, rows), true, nil
}

// ── alias / unalias ───────────────────────────

func builtinAlias(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return builtinListAliases(sh)
	}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			return nil, true, fmt.Errorf("alias: use name=command")
		}
		name := strings.TrimSpace(parts[0])
		expand := strings.Trim(strings.TrimSpace(parts[1]), `'"`)
		sh.aliases[name] = Alias{Name: name, Expand: expand, Created: time.Now()}
	}
	return NewText(""), true, nil
}

func builtinUnalias(sh *Shell, args []string) (*Result, bool, error) {
	for _, a := range args {
		delete(sh.aliases, a)
	}
	return NewText(""), true, nil
}

func builtinListAliases(sh *Shell) (*Result, bool, error) {
	if len(sh.aliases) == 0 {
		return NewText("No aliases defined. Use: alias name=command"), true, nil
	}
	cols := []string{"name", "expands_to", "created"}
	var rows []Row
	for _, a := range sh.aliases {
		rows = append(rows, Row{
			"name":       a.Name,
			"expands_to": a.Expand,
			"created":    a.Created.Format("15:04:05"),
		})
	}
	return NewTable(cols, rows), true, nil
}

// ── history ───────────────────────────────────

func builtinHistory(sh *Shell, args []string) (*Result, bool, error) {
	n := len(sh.history)
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &n)
	}
	start := len(sh.history) - n
	if start < 0 {
		start = 0
	}
	cols := []string{"n", "command", "time"}
	var rows []Row
	for i, entry := range sh.history[start:] {
		rows = append(rows, Row{
			"n":       fmt.Sprintf("%d", start+i+1),
			"command": entry.Raw,
			"time":    entry.At.Format("15:04:05"),
		})
	}
	return NewTable(cols, rows), true, nil
}

// ── help ─────────────────────────────────────

func builtinHelp() (*Result, bool, error) {
	help := `
` + sectionHeader("StructSH — Structured Shell") + `
  Run any OS command. Output is automatically parsed into a table.
  Chain transforms with | pipes. Store results in the Box with #=.

` + c(ansiBold+ansiCyan, "  PIPE OPERATORS") + `

  ` + c(ansiWhite, "| select col1,col2") + `     keep only listed columns
  ` + c(ansiWhite, "| where col=val") + `        filter rows (= != > < >= <= ~)
  ` + c(ansiWhite, "| grep text") + `            search all columns/lines
  ` + c(ansiWhite, "| sort col [asc|desc]") + `  sort by column
  ` + c(ansiWhite, "| limit N") + `              keep first N rows
  ` + c(ansiWhite, "| skip N") + `               skip first N rows
  ` + c(ansiWhite, "| count") + `                count rows / lines
  ` + c(ansiWhite, "| unique [col]") + `         deduplicate rows
  ` + c(ansiWhite, "| reverse") + `              reverse row order
  ` + c(ansiWhite, "| fmt json|csv|tsv") + `     change output format
  ` + c(ansiWhite, "| add col=value") + `        add a column with literal value
  ` + c(ansiWhite, "| rename old=new") + `       rename a column

` + c(ansiBold+ansiCyan, "  BOX STORAGE") + `

  ` + c(ansiWhite, "cmd #=") + `                 store result (auto-name)
  ` + c(ansiWhite, "cmd #=mykey") + `            store result as "mykey"
  ` + c(ansiWhite, "box") + `                    list all entries
  ` + c(ansiWhite, "box get <key|id>") + `       retrieve an entry
  ` + c(ansiWhite, "box rm <key|id>") + `        remove an entry
  ` + c(ansiWhite, "box rename old new") + `     rename an entry
  ` + c(ansiWhite, "box tag <key> <tag>") + `    add a tag
  ` + c(ansiWhite, "box untag <key> <tag>") + `  remove a tag
  ` + c(ansiWhite, "box search <query>") + `     search by name/source
  ` + c(ansiWhite, "box filter tag <tag>") + `   filter by tag
  ` + c(ansiWhite, "box export <file>") + `      export to JSON
  ` + c(ansiWhite, "box import <file>") + `      import from JSON
  ` + c(ansiWhite, "box clear") + `              clear all entries

` + c(ansiBold+ansiCyan, "  FILE COMMANDS") + `

  ` + c(ansiWhite, "ls [-la] [dir]") + `         list directory (structured)
  ` + c(ansiWhite, "cat <file>") + `             show file content
  ` + c(ansiWhite, "head [-n N] <file>") + `     first N lines
  ` + c(ansiWhite, "tail [-n N] <file>") + `     last N lines
  ` + c(ansiWhite, "wc <file>") + `              word/line/byte count
  ` + c(ansiWhite, "stat <file>") + `            file metadata as table
  ` + c(ansiWhite, "find [dir] [-name] [-type]") + ` find files
  ` + c(ansiWhite, "touch / mkdir / rm / cp / mv") + `

` + c(ansiBold+ansiCyan, "  SHELL COMMANDS") + `

  ` + c(ansiWhite, "cd, pwd, pushd, popd") + `
  ` + c(ansiWhite, "echo, set NAME=VAL, unset NAME, vars") + `
  ` + c(ansiWhite, "alias name=cmd, unalias, aliases") + `
  ` + c(ansiWhite, "history [N], clear, help, exit") + `

` + c(ansiBold+ansiCyan, "  EXAMPLES") + `

  ` + c(ansiGrey, "ls -la | where type=dir | select name,size") + `
  ` + c(ansiGrey, "ps | where cpu>5 | sort cpu desc | limit 5 #=top5") + `
  ` + c(ansiGrey, "cat file.csv | grep error | count") + `
  ` + c(ansiGrey, "env | where key~GOPATH | fmt json") + `
  ` + c(ansiGrey, "find . -name *.go | sort path") + `
`
	return NewText(help), true, nil
}

// ── box built-in ─────────────────────────────

func builtinBox(sh *Shell, args []string) (*Result, bool, error) {
	sub := ""
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	switch sub {

	case "get":
		if len(args) < 2 {
			return nil, true, fmt.Errorf("usage: box get <key|id>")
		}
		e, ok := sh.box.Get(args[1])
		if !ok {
			return nil, true, fmt.Errorf("box: no entry %q", args[1])
		}
		sh.printBoxEntry(e)
		return NewText(""), true, nil

	case "rm", "remove", "del":
		if len(args) < 2 {
			return nil, true, fmt.Errorf("usage: box rm <key|id>")
		}
		n := sh.box.Remove(args[1])
		if n == 0 {
			return nil, true, fmt.Errorf("box: no entry %q", args[1])
		}
		fmt.Println(okMsg(fmt.Sprintf("removed %q from box", args[1])))
		return NewText(""), true, nil

	case "rename":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("usage: box rename <old> <new>")
		}
		if !sh.box.Rename(args[1], args[2]) {
			return nil, true, fmt.Errorf("box: no entry %q", args[1])
		}
		fmt.Println(okMsg(fmt.Sprintf("renamed %q → %q", args[1], args[2])))
		return NewText(""), true, nil

	case "tag":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("usage: box tag <key|id> <tag>")
		}
		if !sh.box.Tag(args[1], args[2]) {
			return nil, true, fmt.Errorf("box: no entry %q", args[1])
		}
		fmt.Println(okMsg(fmt.Sprintf("tagged %q with #%s", args[1], args[2])))
		return NewText(""), true, nil

	case "untag":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("usage: box untag <key|id> <tag>")
		}
		sh.box.Untag(args[1], args[2])
		fmt.Println(okMsg(fmt.Sprintf("removed tag #%s from %q", args[2], args[1])))
		return NewText(""), true, nil

	case "search":
		q := ""
		if len(args) > 1 {
			q = args[1]
		}
		entries := sh.box.List(q, "")
		if len(entries) == 0 {
			fmt.Println(infoMsg(fmt.Sprintf("no entries matching %q", q)))
			return NewText(""), true, nil
		}
		sh.printBoxList(entries)
		return NewText(""), true, nil

	case "filter":
		if len(args) < 3 {
			return nil, true, fmt.Errorf("usage: box filter tag <tagname>")
		}
		entries := sh.box.List("", args[2])
		if len(entries) == 0 {
			fmt.Println(infoMsg(fmt.Sprintf("no entries tagged #%s", args[2])))
			return NewText(""), true, nil
		}
		sh.printBoxList(entries)
		return NewText(""), true, nil

	case "export":
		if len(args) < 2 {
			return nil, true, fmt.Errorf("usage: box export <file.json>")
		}
		p := resolvePath(sh.cwd, args[1])
		if err := sh.box.ExportJSON(p); err != nil {
			return nil, true, err
		}
		fmt.Println(okMsg(fmt.Sprintf("exported %d entries to %s", sh.box.Len(), args[1])))
		return NewText(""), true, nil

	case "import":
		if len(args) < 2 {
			return nil, true, fmt.Errorf("usage: box import <file.json>")
		}
		p := resolvePath(sh.cwd, args[1])
		n, err := sh.box.ImportJSON(p)
		if err != nil {
			return nil, true, err
		}
		fmt.Println(okMsg(fmt.Sprintf("imported %d entries from %s", n, args[1])))
		return NewText(""), true, nil

	case "clear":
		sh.box.Clear()
		fmt.Println(warnMsg("box cleared"))
		return NewText(""), true, nil

	default:
		entries := sh.box.List("", "")
		if len(entries) == 0 {
			fmt.Println(c(ansiGrey, `
  Box is empty.
  Store results with: cmd #=name   or   cmd #=`))
			return NewText(""), true, nil
		}
		sh.printBoxList(entries)
		return NewText(""), true, nil
	}
}

// ── helpers ───────────────────────────────────

func resolvePath(cwd, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir(), p[2:])
	}
	return filepath.Clean(filepath.Join(cwd, p))
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "/"
}

func findInPath(name string) string {
	pathDirs := strings.Split(os.Getenv("PATH"), ":")
	for _, dir := range pathDirs {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}
