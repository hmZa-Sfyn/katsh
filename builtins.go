package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Built-in commands — all as package main
//
//  Covered (50 most-used bash commands + shell builtins):
//   Navigation : cd, pwd, pushd, popd, dirs
//   Listing    : ls, tree, du
//   Files      : cat, head, tail, touch, cp, mv, rm, mkdir, rmdir, ln
//   Viewing    : wc, stat, file, find, locate (indexed find), diff
//   Search     : grep, sed (replace), awk (field extract), cut, tr
//   Text       : sort (standalone), uniq, tee, split
//   Perms      : chmod, chown (metadata only in pure Go)
//   Process    : ps, kill, jobs (placeholder), sleep
//   Sys info   : uname, uptime, date, cal, hostname, whoami, id, groups
//   Network    : ping, curl (wget), nslookup, ifconfig/ip
//   Archiving  : tar (via exec), gzip (via exec), zip (via exec)
//   Hash       : md5sum, sha1sum, sha256sum
//   Misc       : echo, printf, read (interactive), yes, seq, base64,
//                xargs (structured), tee, watch (simple), type,
//                source, export, which, man (short desc), true, false
//   Shell      : alias, unalias, aliases, set, unset, vars,
//                history, clear, help, exit
//   Box        : box (all sub-commands)
// ─────────────────────────────────────────────────────────────────────────────

// handleBuiltin checks if cmd is a built-in and runs it.
// Returns (result, wasBuiltin, error).
func handleBuiltin(sh *Shell, command string, args []string) (*Result, bool, error) {
	switch command {

	// ── Navigation ──────────────────────────────────────────────────────────
	case "cd":
		return builtinCD(sh, args)
	case "pwd":
		return NewText(sh.cwd), true, nil
	case "pushd":
		return builtinPushd(sh, args)
	case "popd":
		return builtinPopd(sh)
	case "dirs":
		return builtinDirs(sh)

	// ── Directory listing ───────────────────────────────────────────────────
	case "ls", "ll", "la":
		if command == "ll" {
			args = append([]string{"-l"}, args...)
		} else if command == "la" {
			args = append([]string{"-la"}, args...)
		}
		return builtinLS(sh, args)
	case "tree":
		return builtinTree(sh, args)
	case "du":
		return builtinDU(sh, args)
	case "df":
		return builtinDF(sh, args)

	// ── File operations ─────────────────────────────────────────────────────
	case "cat":
		return builtinCat(sh, args)
	case "head":
		return builtinHead(sh, args)
	case "tail":
		return builtinTail(sh, args)
	case "touch":
		return builtinTouch(sh, args)
	case "mkdir":
		return builtinMkdir(sh, args)
	case "rmdir":
		return builtinRmdir(sh, args)
	case "rm":
		return builtinRm(sh, args)
	case "cp":
		return builtinCp(sh, args)
	case "mv":
		return builtinMv(sh, args)
	case "ln":
		return builtinLn(sh, args)

	// ── File inspection ─────────────────────────────────────────────────────
	case "wc":
		return builtinWC(sh, args)
	case "stat":
		return builtinStat(sh, args)
	case "file":
		return builtinFile(sh, args)
	case "find":
		return builtinFind(sh, args)
	case "diff":
		return builtinDiff(sh, args)

	// ── Text processing ─────────────────────────────────────────────────────
	case "grep":
		return builtinGrep(sh, args)
	case "sed":
		return builtinSed(sh, args)
	case "awk":
		return builtinAwk(sh, args)
	case "cut":
		return builtinCut(sh, args)
	case "tr":
		return builtinTr(sh, args)
	case "sort":
		return builtinSort(sh, args)
	case "uniq":
		return builtinUniq(sh, args)
	case "tee":
		return builtinTee(sh, args)
	case "split":
		return builtinSplit(sh, args)
	case "xargs":
		return builtinXargs(sh, args)

	// ── Permissions ─────────────────────────────────────────────────────────
	case "chmod":
		return builtinChmod(sh, args)
	case "chown":
		return builtinChown(sh, args)

	// ── Process ─────────────────────────────────────────────────────────────
	case "ps":
		return builtinPS(sh, args)
	case "kill":
		return builtinKill(sh, args)
	case "sleep":
		return builtinSleep(sh, args)
	case "jobs":
		return NewText("(no background jobs)"), true, nil

	// ── System info ─────────────────────────────────────────────────────────
	case "uname":
		return builtinUname(sh, args)
	case "uptime":
		return builtinUptime(sh, args)
	case "date":
		return builtinDate(sh, args)
	case "cal":
		return builtinCal(sh, args)
	case "hostname":
		return builtinHostname(sh, args)
	case "whoami":
		name, _ := osUsername()
		return NewText(name), true, nil
	case "id":
		return builtinID(sh, args)
	case "groups":
		return builtinGroups(sh, args)
	case "w", "who":
		return builtinWho(sh, args)

	// ── Network ─────────────────────────────────────────────────────────────
	case "ping":
		return builtinPing(sh, args)
	case "curl", "wget":
		return builtinCurl(sh, args)
	case "nslookup", "dig":
		return builtinNslookup(sh, args)
	case "ifconfig", "ip":
		return builtinIfconfig(sh, args)

	// ── Hashing ─────────────────────────────────────────────────────────────
	case "md5sum", "md5":
		return builtinHash(sh, args, "md5")
	case "sha1sum", "sha1":
		return builtinHash(sh, args, "sha1")
	case "sha256sum", "sha256":
		return builtinHash(sh, args, "sha256")

	// ── Archiving (delegate to system) ──────────────────────────────────────
	case "tar", "gzip", "gunzip", "zip", "unzip":
		return builtinSysDelegate(sh, command, args)

	// ── Text generation & encoding ──────────────────────────────────────────
	case "echo":
		return builtinEcho(sh, args)
	case "printf":
		return builtinPrintf(sh, args)
	case "yes":
		return builtinYes(sh, args)
	case "seq":
		return builtinSeq(sh, args)
	case "base64":
		return builtinBase64(sh, args)
	case "rev":
		return builtinRev(sh, args)

	// ── Shell variables & environment ────────────────────────────────────────
	case "set":
		return builtinSet(sh, args)
	case "unset":
		return builtinUnset(sh, args)
	case "vars":
		return builtinVars(sh)
	case "export":
		return builtinExport(sh, args)
	case "env":
		return builtinEnv(sh, args)
	case "printenv":
		return builtinPrintenv(sh, args)

	// ── Alias ────────────────────────────────────────────────────────────────
	case "alias":
		return builtinAlias(sh, args)
	case "unalias":
		return builtinUnalias(sh, args)
	case "aliases":
		return builtinListAliases(sh)

	// ── Identification ────────────────────────────────────────────────────────
	case "which":
		return builtinWhich(sh, args)
	case "type":
		return builtinType(sh, args)

	// ── Numeric & misc utils ─────────────────────────────────────────────────
	case "bc":
		return builtinBC(sh, args)
	case "factor":
		return builtinFactor(sh, args)
	case "random":
		return builtinRandom(sh, args)

	// ── Box ──────────────────────────────────────────────────────────────────
	case "box":
		return builtinBox(sh, args)

	// ── Session ──────────────────────────────────────────────────────────────
	case "history":
		return builtinHistory(sh, args)
	case "clear":
		return nil, true, errClear
	case "help":
		return builtinHelp()
	case "man":
		return builtinMan(sh, args)
	case "true":
		return NewText(""), true, nil
	case "false":
		return nil, true, fmt.Errorf("false")
	case "exit", "quit":
		return nil, true, errExit
	case "source", ".":
		return builtinSource(sh, args)
	case "watch":
		return builtinWatch(sh, args)
	}

	return nil, false, nil
}

// sentinel errors
var errExit = fmt.Errorf("__exit__")
var errClear = fmt.Errorf("__clear__")

// ═════════════════════════════════════════════════════════════════════════════
//  NAVIGATION
// ═════════════════════════════════════════════════════════════════════════════

func builtinCD(sh *Shell, args []string) (*Result, bool, error) {
	target := homeDir()
	if len(args) > 0 {
		target = args[0]
	}
	if target == "~" || target == "" {
		target = homeDir()
	} else if strings.HasPrefix(target, "~/") {
		target = filepath.Join(homeDir(), target[2:])
	} else if target == "-" {
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
	return builtinDirs(sh)
}

func builtinPopd(sh *Shell) (*Result, bool, error) {
	if len(sh.dirStack) == 0 {
		return nil, true, fmt.Errorf("popd: directory stack empty")
	}
	n := len(sh.dirStack)
	prev := sh.dirStack[n-1]
	sh.dirStack = sh.dirStack[:n-1]
	_, _, err := builtinCD(sh, []string{prev})
	if err != nil {
		return nil, true, err
	}
	return builtinDirs(sh)
}

func builtinDirs(sh *Shell) (*Result, bool, error) {
	cols := []string{"index", "path"}
	var rows []Row
	rows = append(rows, Row{"index": "0 (current)", "path": sh.cwd})
	for i := len(sh.dirStack) - 1; i >= 0; i-- {
		rows = append(rows, Row{
			"index": fmt.Sprintf("%d", len(sh.dirStack)-i),
			"path":  sh.dirStack[i],
		})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  DIRECTORY LISTING
// ═════════════════════════════════════════════════════════════════════════════

func builtinLS(sh *Shell, args []string) (*Result, bool, error) {
	long := false
	all := false
	human := true
	dir := sh.cwd

	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "l") {
				long = true
			}
			if strings.Contains(a, "a") {
				all = true
			}
			if strings.Contains(a, "h") {
				human = true
			}
		} else {
			dir = resolvePath(sh.cwd, a)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, true, fmt.Errorf("ls: %s: %w", dir, err)
	}

	var cols []string
	var rows []Row

	if long {
		cols = []string{"perms", "size", "modified", "type", "name"}
		for _, e := range entries {
			if !all && strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			t := "file"
			name := e.Name()
			if e.IsDir() {
				t = "dir"
				name += "/"
			} else if info.Mode()&os.ModeSymlink != 0 {
				t = "symlink"
			}
			sz := ""
			if human {
				sz = fmtBytes(info.Size())
			} else {
				sz = strconv.FormatInt(info.Size(), 10)
			}
			rows = append(rows, Row{
				"perms":    info.Mode().String(),
				"size":     sz,
				"modified": info.ModTime().Format("2006-01-02 15:04"),
				"type":     t,
				"name":     name,
			})
		}
	} else {
		cols = []string{"name", "type", "size"}
		for _, e := range entries {
			if !all && strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, _ := e.Info()
			t := "file"
			name := e.Name()
			size := ""
			if e.IsDir() {
				t = "dir"
				name += "/"
			} else if info != nil {
				size = fmtBytes(info.Size())
			}
			rows = append(rows, Row{"name": name, "type": t, "size": size})
		}
	}
	return NewTable(cols, rows), true, nil
}

func builtinTree(sh *Shell, args []string) (*Result, bool, error) {
	dir := sh.cwd
	maxDepth := 3
	for i := 0; i < len(args); i++ {
		if args[i] == "-L" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &maxDepth)
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			dir = resolvePath(sh.cwd, args[i])
		}
	}
	var lines []string
	treeWalk(dir, "", maxDepth, 0, &lines)
	lines = append([]string{dir}, lines...)
	return NewText(strings.Join(lines, "\n")), true, nil
}

func treeWalk(dir, prefix string, maxDepth, depth int, lines *[]string) {
	if depth >= maxDepth {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for i, e := range entries {
		connector := "├── "
		newPrefix := prefix + "│   "
		if i == len(entries)-1 {
			connector = "└── "
			newPrefix = prefix + "    "
		}
		*lines = append(*lines, prefix+connector+e.Name())
		if e.IsDir() {
			treeWalk(filepath.Join(dir, e.Name()), newPrefix, maxDepth, depth+1, lines)
		}
	}
}

func builtinDU(sh *Shell, args []string) (*Result, bool, error) {
	human := true
	summarize := false
	dir := sh.cwd
	for _, a := range args {
		if a == "-s" || a == "--summarize" {
			summarize = true
		} else if a == "-b" {
			human = false
		} else if !strings.HasPrefix(a, "-") {
			dir = resolvePath(sh.cwd, a)
		}
	}

	cols := []string{"size", "path"}
	var rows []Row

	if summarize {
		size := dirSize(dir)
		sz := strconv.FormatInt(size, 10)
		if human {
			sz = fmtBytes(size)
		}
		rows = append(rows, Row{"size": sz, "path": dir})
		return NewTable(cols, rows), true, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, true, fmt.Errorf("du: %w", err)
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		var size int64
		if e.IsDir() {
			size = dirSize(full)
		} else {
			info, _ := e.Info()
			if info != nil {
				size = info.Size()
			}
		}
		sz := strconv.FormatInt(size, 10)
		if human {
			sz = fmtBytes(size)
		}
		rows = append(rows, Row{"size": sz, "path": e.Name()})
	}
	return NewTable(cols, rows), true, nil
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func builtinDF(sh *Shell, args []string) (*Result, bool, error) {
	out, err := rawExec("df", append([]string{"-h"}, args...), "")
	if err != nil {
		return nil, true, fmt.Errorf("df: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return NewText(out), true, nil
	}
	cols := []string{"filesystem", "size", "used", "avail", "use%", "mounted"}
	var rows []Row
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		rows = append(rows, Row{
			"filesystem": f[0],
			"size":       f[1],
			"used":       f[2],
			"avail":      f[3],
			"use%":       f[4],
			"mounted":    f[5],
		})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  FILE OPERATIONS
// ═════════════════════════════════════════════════════════════════════════════

func builtinCat(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("cat: missing file argument")
	}
	showLineNumbers := false
	var files []string
	for _, a := range args {
		if a == "-n" {
			showLineNumbers = true
		} else {
			files = append(files, a)
		}
	}
	var parts []string
	for _, arg := range files {
		p := resolvePath(sh.cwd, arg)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("cat: %s: %w", arg, err)
		}
		content := string(data)
		if showLineNumbers {
			lines := strings.Split(content, "\n")
			for i, l := range lines {
				lines[i] = fmt.Sprintf("%6d  %s", i+1, l)
			}
			content = strings.Join(lines, "\n")
		}
		parts = append(parts, content)
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
		} else if strings.HasPrefix(args[i], "-") && len(args[i]) > 1 {
			// -20 style
			fmt.Sscanf(args[i][1:], "%d", &n)
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
		} else if strings.HasPrefix(args[i], "-") && len(args[i]) > 1 && args[i] != "-f" {
			fmt.Sscanf(args[i][1:], "%d", &n)
		} else if args[i] != "-f" {
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

func builtinTouch(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("touch: missing file argument")
	}
	var created, updated []string
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, []byte{}, 0644); err != nil {
				return nil, true, fmt.Errorf("touch: %s: %w", f, err)
			}
			created = append(created, f)
		} else {
			now := time.Now()
			if err := os.Chtimes(p, now, now); err != nil {
				return nil, true, fmt.Errorf("touch: %s: %w", f, err)
			}
			updated = append(updated, f)
		}
	}
	cols := []string{"action", "file"}
	var rows []Row
	for _, f := range created {
		rows = append(rows, Row{"action": "created", "file": f})
	}
	for _, f := range updated {
		rows = append(rows, Row{"action": "updated", "file": f})
	}
	return NewTable(cols, rows), true, nil
}

func builtinMkdir(sh *Shell, args []string) (*Result, bool, error) {
	parents := false
	var dirs []string
	for _, a := range args {
		if a == "-p" || a == "--parents" {
			parents = true
		} else {
			dirs = append(dirs, a)
		}
	}
	if len(dirs) == 0 {
		return nil, true, fmt.Errorf("mkdir: missing operand")
	}
	cols := []string{"status", "path"}
	var rows []Row
	for _, d := range dirs {
		p := resolvePath(sh.cwd, d)
		var err error
		if parents {
			err = os.MkdirAll(p, 0755)
		} else {
			err = os.Mkdir(p, 0755)
		}
		status := "created"
		if err != nil {
			status = "error: " + err.Error()
		}
		rows = append(rows, Row{"status": status, "path": d})
	}
	return NewTable(cols, rows), true, nil
}

func builtinRmdir(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("rmdir: missing operand")
	}
	cols := []string{"status", "path"}
	var rows []Row
	for _, d := range args {
		p := resolvePath(sh.cwd, d)
		err := os.Remove(p)
		status := "removed"
		if err != nil {
			status = "error: " + err.Error()
		}
		rows = append(rows, Row{"status": status, "path": d})
	}
	return NewTable(cols, rows), true, nil
}

func builtinRm(sh *Shell, args []string) (*Result, bool, error) {
	recursive := false
	force := false
	verbose := false
	var targets []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "r") || strings.Contains(a, "R") {
				recursive = true
			}
			if strings.Contains(a, "f") {
				force = true
			}
			if strings.Contains(a, "v") {
				verbose = true
			}
		} else {
			targets = append(targets, a)
		}
	}
	if len(targets) == 0 {
		return nil, true, fmt.Errorf("rm: missing operand")
	}
	cols := []string{"status", "path"}
	var rows []Row
	for _, t := range targets {
		p := resolvePath(sh.cwd, t)
		info, statErr := os.Stat(p)
		if os.IsNotExist(statErr) {
			if force {
				continue
			}
			return nil, true, fmt.Errorf("rm: %s: no such file or directory", t)
		}
		if info.IsDir() && !recursive {
			return nil, true, fmt.Errorf("rm: %s: is a directory (use -r)", t)
		}
		var err error
		if recursive {
			err = os.RemoveAll(p)
		} else {
			err = os.Remove(p)
		}
		if err != nil {
			return nil, true, fmt.Errorf("rm: %s: %w", t, err)
		}
		if verbose {
			rows = append(rows, Row{"status": "removed", "path": t})
		}
	}
	if verbose {
		return NewTable(cols, rows), true, nil
	}
	return NewText(""), true, nil
}

func builtinCp(sh *Shell, args []string) (*Result, bool, error) {
	recursive := false
	verbose := false
	var operands []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "r") || strings.Contains(a, "R") {
				recursive = true
			}
			if strings.Contains(a, "v") {
				verbose = true
			}
		} else {
			operands = append(operands, a)
		}
	}
	if len(operands) < 2 {
		return nil, true, fmt.Errorf("cp: usage: cp [-rv] <src> <dst>")
	}
	src := resolvePath(sh.cwd, operands[0])
	dst := resolvePath(sh.cwd, operands[1])

	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, true, fmt.Errorf("cp: %w", err)
	}
	if srcInfo.IsDir() {
		if !recursive {
			return nil, true, fmt.Errorf("cp: %s: is a directory (use -r)", operands[0])
		}
		err = copyDir(src, dst)
	} else {
		err = copyFile(src, dst)
	}
	if err != nil {
		return nil, true, fmt.Errorf("cp: %w", err)
	}
	if verbose {
		return NewText(fmt.Sprintf("'%s' -> '%s'", operands[0], operands[1])), true, nil
	}
	return NewText(""), true, nil
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

func builtinMv(sh *Shell, args []string) (*Result, bool, error) {
	verbose := false
	var operands []string
	for _, a := range args {
		if a == "-v" {
			verbose = true
		} else {
			operands = append(operands, a)
		}
	}
	if len(operands) < 2 {
		return nil, true, fmt.Errorf("mv: usage: mv <src> <dst>")
	}
	src := resolvePath(sh.cwd, operands[0])
	dst := resolvePath(sh.cwd, operands[1])
	if err := os.Rename(src, dst); err != nil {
		return nil, true, fmt.Errorf("mv: %w", err)
	}
	if verbose {
		return NewText(fmt.Sprintf("'%s' -> '%s'", operands[0], operands[1])), true, nil
	}
	return NewText(""), true, nil
}

func builtinLn(sh *Shell, args []string) (*Result, bool, error) {
	symbolic := false
	var operands []string
	for _, a := range args {
		if a == "-s" {
			symbolic = true
		} else {
			operands = append(operands, a)
		}
	}
	if len(operands) < 2 {
		return nil, true, fmt.Errorf("ln: usage: ln [-s] <target> <link>")
	}
	target := resolvePath(sh.cwd, operands[0])
	link := resolvePath(sh.cwd, operands[1])
	var err error
	if symbolic {
		err = os.Symlink(target, link)
	} else {
		err = os.Link(target, link)
	}
	if err != nil {
		return nil, true, fmt.Errorf("ln: %w", err)
	}
	return NewText(""), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  FILE INSPECTION
// ═════════════════════════════════════════════════════════════════════════════

func builtinWC(sh *Shell, args []string) (*Result, bool, error) {
	onlyLines := false
	onlyWords := false
	onlyBytes := false
	var files []string
	for _, a := range args {
		switch a {
		case "-l":
			onlyLines = true
		case "-w":
			onlyWords = true
		case "-c", "-m":
			onlyBytes = true
		default:
			if !strings.HasPrefix(a, "-") {
				files = append(files, a)
			}
		}
	}
	if len(files) == 0 {
		return nil, true, fmt.Errorf("wc: missing file argument")
	}
	// Build column list
	cols := []string{}
	if !onlyWords && !onlyBytes {
		cols = append(cols, "lines")
	}
	if !onlyLines && !onlyBytes {
		cols = append(cols, "words")
	}
	if !onlyLines && !onlyWords {
		cols = append(cols, "bytes")
	}
	if len(cols) == 0 {
		cols = []string{"lines", "words", "bytes"}
	}
	cols = append(cols, "file")

	var rows []Row
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("wc: %s: %w", f, err)
		}
		nlines := strings.Count(string(data), "\n")
		nwords := len(strings.Fields(string(data)))
		nbytes := len(data)
		row := Row{"file": f}
		row["lines"] = strconv.Itoa(nlines)
		row["words"] = strconv.Itoa(nwords)
		row["bytes"] = strconv.Itoa(nbytes)
		rows = append(rows, row)
	}
	return NewTable(cols, rows), true, nil
}

func builtinStat(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("stat: missing file argument")
	}
	cols := []string{"name", "size", "mode", "modified", "type"}
	var rows []Row
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		info, err := os.Lstat(p)
		if err != nil {
			return nil, true, fmt.Errorf("stat: %s: %w", f, err)
		}
		ftype := "file"
		if info.IsDir() {
			ftype = "dir"
		} else if info.Mode()&os.ModeSymlink != 0 {
			ftype = "symlink"
		}
		rows = append(rows, Row{
			"name":     f,
			"size":     fmtBytes(info.Size()),
			"mode":     info.Mode().String(),
			"modified": info.ModTime().Format("2006-01-02 15:04:05"),
			"type":     ftype,
		})
	}
	return NewTable(cols, rows), true, nil
}

func builtinFile(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("file: missing argument")
	}
	cols := []string{"name", "description"}
	var rows []Row
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		info, err := os.Lstat(p)
		if err != nil {
			rows = append(rows, Row{"name": f, "description": "cannot stat: " + err.Error()})
			continue
		}
		desc := sniffFileType(p, info)
		rows = append(rows, Row{"name": f, "description": desc})
	}
	return NewTable(cols, rows), true, nil
}

// sniffFileType makes a basic determination of file type.
func sniffFileType(path string, info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(path)
		return "symbolic link → " + target
	}
	if info.Mode()&0111 != 0 {
		return "executable"
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "Go source code"
	case ".py":
		return "Python script"
	case ".sh", ".bash":
		return "shell script"
	case ".json":
		return "JSON data"
	case ".yaml", ".yml":
		return "YAML document"
	case ".toml":
		return "TOML config"
	case ".md":
		return "Markdown text"
	case ".txt":
		return "plain text"
	case ".html", ".htm":
		return "HTML document"
	case ".css":
		return "CSS stylesheet"
	case ".js", ".ts":
		return "JavaScript/TypeScript source"
	case ".png":
		return "PNG image"
	case ".jpg", ".jpeg":
		return "JPEG image"
	case ".gif":
		return "GIF image"
	case ".svg":
		return "SVG vector image"
	case ".pdf":
		return "PDF document"
	case ".zip":
		return "Zip archive"
	case ".tar":
		return "tar archive"
	case ".gz":
		return "gzip compressed"
	case ".sql":
		return "SQL script"
	case ".csv":
		return "CSV spreadsheet"
	case ".xml":
		return "XML document"
	case ".log":
		return "log file"
	case ".mod":
		return "Go module file"
	case ".sum":
		return "Go checksum file"
	}
	// Peek at first bytes
	f, err := os.Open(path)
	if err != nil {
		return "data"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "empty"
	}
	// Check for binary
	for _, b := range buf[:n] {
		if b == 0 {
			return "binary data"
		}
	}
	return "ASCII text"
}

func builtinFind(sh *Shell, args []string) (*Result, bool, error) {
	dir := sh.cwd
	namePattern := ""
	typeFilter := ""
	maxDepth := 20
	minSize := int64(-1)
	newerThan := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-name", "-iname":
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
		case "-size":
			if i+1 < len(args) {
				s := args[i+1]
				s = strings.TrimSuffix(s, "k")
				s = strings.TrimSuffix(s, "M")
				fmt.Sscanf(s, "%d", &minSize)
				i++
			}
		case "-newer":
			if i+1 < len(args) {
				newerThan = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				dir = resolvePath(sh.cwd, args[i])
			}
		}
	}

	var newerTime time.Time
	if newerThan != "" {
		p := resolvePath(sh.cwd, newerThan)
		if info, err := os.Stat(p); err == nil {
			newerTime = info.ModTime()
		}
	}

	cols := []string{"path", "type", "size", "modified"}
	var rows []Row
	_ = walkFind(dir, dir, namePattern, typeFilter, maxDepth, 0, &rows, minSize, newerTime)
	if len(rows) == 0 {
		return NewText("(no matches)"), true, nil
	}
	return NewTable(cols, rows), true, nil
}

func walkFind(root, dir, namePattern, typeFilter string, maxDepth, depth int, rows *[]Row, minSize int64, newerThan time.Time) error {
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

		info, _ := e.Info()

		// Type filter
		if typeFilter != "" {
			if typeFilter == "f" && t != "file" {
				goto descend
			}
			if typeFilter == "d" && t != "dir" {
				goto descend
			}
		}
		// Name filter
		if namePattern != "" {
			matched, _ := filepath.Match(namePattern, e.Name())
			if !matched {
				matched = strings.Contains(strings.ToLower(e.Name()), strings.ToLower(strings.Trim(namePattern, "*")))
				if !matched {
					goto descend
				}
			}
		}
		// Size filter
		if minSize >= 0 && info != nil && info.Size() < minSize {
			goto descend
		}
		// Newer filter
		if !newerThan.IsZero() && info != nil && !info.ModTime().After(newerThan) {
			goto descend
		}

		{
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
			_ = walkFind(root, fullPath, namePattern, typeFilter, maxDepth, depth+1, rows, minSize, newerThan)
		}
	}
	return nil
}

func builtinDiff(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 2 {
		return nil, true, fmt.Errorf("diff: usage: diff <file1> <file2>")
	}
	p1 := resolvePath(sh.cwd, args[0])
	p2 := resolvePath(sh.cwd, args[1])
	d1, err := os.ReadFile(p1)
	if err != nil {
		return nil, true, fmt.Errorf("diff: %w", err)
	}
	d2, err := os.ReadFile(p2)
	if err != nil {
		return nil, true, fmt.Errorf("diff: %w", err)
	}
	l1 := strings.Split(string(d1), "\n")
	l2 := strings.Split(string(d2), "\n")
	cols := []string{"line", "status", "content"}
	var rows []Row
	maxLen := len(l1)
	if len(l2) > maxLen {
		maxLen = len(l2)
	}
	for i := 0; i < maxLen; i++ {
		lineNum := strconv.Itoa(i + 1)
		var a, b string
		if i < len(l1) {
			a = l1[i]
		}
		if i < len(l2) {
			b = l2[i]
		}
		if a == b {
			rows = append(rows, Row{"line": lineNum, "status": "=", "content": a})
		} else {
			if i < len(l1) {
				rows = append(rows, Row{"line": lineNum, "status": "< " + args[0], "content": a})
			}
			if i < len(l2) {
				rows = append(rows, Row{"line": lineNum, "status": "> " + args[1], "content": b})
			}
		}
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  TEXT PROCESSING
// ═════════════════════════════════════════════════════════════════════════════

func builtinGrep(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("grep: usage: grep [-inrv] <pattern> [file...]")
	}
	caseInsensitive := false
	showLineNumbers := false
	invertMatch := false
	recursive := false
	var pattern string
	var files []string

	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags := args[i][1:]
			if strings.Contains(flags, "i") {
				caseInsensitive = true
			}
			if strings.Contains(flags, "n") {
				showLineNumbers = true
			}
			if strings.Contains(flags, "v") {
				invertMatch = true
			}
			if strings.Contains(flags, "r") || strings.Contains(flags, "R") {
				recursive = true
			}
		} else if pattern == "" {
			pattern = args[i]
		} else {
			files = append(files, args[i])
		}
	}
	if pattern == "" {
		return nil, true, fmt.Errorf("grep: missing pattern")
	}

	rxStr := pattern
	if caseInsensitive {
		rxStr = "(?i)" + pattern
	}
	re, err := regexp.Compile(rxStr)
	if err != nil {
		return nil, true, fmt.Errorf("grep: invalid pattern: %w", err)
	}

	if len(files) == 0 {
		return nil, true, fmt.Errorf("grep: no files specified (stdin not supported in StructSH — use | grep in pipes)")
	}

	// Expand recursive
	if recursive {
		var expanded []string
		for _, f := range files {
			p := resolvePath(sh.cwd, f)
			_ = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					expanded = append(expanded, path)
				}
				return nil
			})
		}
		files = expanded
	}

	cols := []string{"file", "line", "content"}
	var rows []Row
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(data), "\n") {
			matched := re.MatchString(line)
			if invertMatch {
				matched = !matched
			}
			if matched {
				row := Row{"file": f, "line": strconv.Itoa(i + 1), "content": line}
				if !showLineNumbers {
					delete(row, "line")
				}
				rows = append(rows, row)
			}
		}
	}
	if !showLineNumbers {
		cols = []string{"file", "content"}
	}
	return NewTable(cols, rows), true, nil
}

func builtinSed(sh *Shell, args []string) (*Result, bool, error) {
	// Simplified sed: supports s/old/new/[g] and d (delete matching lines)
	if len(args) < 2 {
		return nil, true, fmt.Errorf("sed: usage: sed 's/old/new/[g]' <file>")
	}
	expr := args[0]
	var files []string
	for _, a := range args[1:] {
		if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		return nil, true, fmt.Errorf("sed: no files specified")
	}

	var results []string
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("sed: %s: %w", f, err)
		}
		out, err := sedApply(string(data), expr)
		if err != nil {
			return nil, true, fmt.Errorf("sed: %w", err)
		}
		results = append(results, out)
	}
	return NewText(strings.Join(results, "\n")), true, nil
}

func sedApply(text, expr string) (string, error) {
	lines := strings.Split(text, "\n")
	var out []string

	// Delete command: /pattern/d
	if strings.HasSuffix(expr, "/d") {
		parts := strings.SplitN(expr, "/", 3)
		if len(parts) >= 2 {
			re, err := regexp.Compile(parts[1])
			if err != nil {
				return "", err
			}
			for _, line := range lines {
				if !re.MatchString(line) {
					out = append(out, line)
				}
			}
			return strings.Join(out, "\n"), nil
		}
	}

	// Substitute: s/old/new/ or s/old/new/g
	if strings.HasPrefix(expr, "s/") || strings.HasPrefix(expr, "s,") {
		sep := string(expr[1])
		parts := strings.SplitN(expr[2:], sep, 3)
		if len(parts) < 3 {
			return "", fmt.Errorf("invalid sed expression: %q", expr)
		}
		pattern := parts[0]
		replacement := parts[1]
		flags := parts[2]
		global := strings.Contains(flags, "g")

		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid pattern: %w", err)
		}
		for _, line := range lines {
			if global {
				out = append(out, re.ReplaceAllString(line, replacement))
			} else {
				out = append(out, re.ReplaceAllLiteralString(line, replacement))
			}
		}
		return strings.Join(out, "\n"), nil
	}

	return text, fmt.Errorf("unsupported sed expression: %q", expr)
}

func builtinAwk(sh *Shell, args []string) (*Result, bool, error) {
	// Structured awk: extracts fields and presents as table
	// Usage: awk '{print $1,$3}' file  OR  awk -F: '{print $1}' file
	if len(args) < 2 {
		return nil, true, fmt.Errorf("awk: usage: awk [-F sep] '{print $N,...}' <file>")
	}
	sep := " "
	var program string
	var files []string

	for i := 0; i < len(args); i++ {
		if args[i] == "-F" && i+1 < len(args) {
			sep = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "{") || strings.HasPrefix(args[i], "'") {
			program = strings.Trim(args[i], "'")
		} else {
			files = append(files, args[i])
		}
	}
	if program == "" {
		return nil, true, fmt.Errorf("awk: missing program")
	}

	// Parse field indices from {print $1,$2,...}
	re := regexp.MustCompile(`\$(\d+)`)
	matches := re.FindAllStringSubmatch(program, -1)
	var fieldIdxs []int
	for _, m := range matches {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		fieldIdxs = append(fieldIdxs, n)
	}
	if len(fieldIdxs) == 0 {
		return nil, true, fmt.Errorf("awk: no field references found (use $1, $2, ...)")
	}

	// Build col names
	cols := make([]string, len(fieldIdxs))
	for i, fi := range fieldIdxs {
		cols[i] = fmt.Sprintf("$%d", fi)
	}

	var rows []Row
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("awk: %s: %w", f, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fields := strings.Split(line, sep)
			if sep == " " {
				fields = strings.Fields(line)
			}
			row := make(Row)
			for i, fi := range fieldIdxs {
				val := ""
				if fi-1 < len(fields) && fi > 0 {
					val = fields[fi-1]
				}
				row[cols[i]] = val
			}
			rows = append(rows, row)
		}
	}
	return NewTable(cols, rows), true, nil
}

func builtinCut(sh *Shell, args []string) (*Result, bool, error) {
	delim := "\t"
	var fieldSpec string
	var files []string

	for i := 0; i < len(args); i++ {
		if args[i] == "-d" && i+1 < len(args) {
			delim = args[i+1]
			i++
		} else if args[i] == "-f" && i+1 < len(args) {
			fieldSpec = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "-d") {
			delim = args[i][2:]
		} else if strings.HasPrefix(args[i], "-f") {
			fieldSpec = args[i][2:]
		} else if !strings.HasPrefix(args[i], "-") {
			files = append(files, args[i])
		}
	}
	if fieldSpec == "" {
		return nil, true, fmt.Errorf("cut: -f field list required")
	}

	// Parse field spec: "1,3,5" or "1-3"
	var fields []int
	for _, part := range strings.Split(fieldSpec, ",") {
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			var lo, hi int
			fmt.Sscanf(bounds[0], "%d", &lo)
			fmt.Sscanf(bounds[1], "%d", &hi)
			for i := lo; i <= hi; i++ {
				fields = append(fields, i)
			}
		} else {
			var n int
			fmt.Sscanf(part, "%d", &n)
			fields = append(fields, n)
		}
	}

	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = fmt.Sprintf("f%d", f)
	}

	var rows []Row
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("cut: %s: %w", f, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, delim)
			row := make(Row, len(fields))
			for i, fi := range fields {
				val := ""
				if fi-1 < len(parts) && fi > 0 {
					val = parts[fi-1]
				}
				row[cols[i]] = val
			}
			rows = append(rows, row)
		}
	}
	return NewTable(cols, rows), true, nil
}

func builtinTr(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 3 {
		return nil, true, fmt.Errorf("tr: usage: tr <set1> <set2> <file>")
	}
	set1 := args[0]
	set2 := args[1]
	file := args[2]

	// Handle escape sequences
	set1 = strings.ReplaceAll(set1, `\n`, "\n")
	set1 = strings.ReplaceAll(set1, `\t`, "\t")
	set2 = strings.ReplaceAll(set2, `\n`, "\n")
	set2 = strings.ReplaceAll(set2, `\t`, "\t")

	p := resolvePath(sh.cwd, file)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, true, fmt.Errorf("tr: %w", err)
	}

	text := string(data)
	// Build translation table
	for i, ch := range set1 {
		if i < len([]rune(set2)) {
			text = strings.ReplaceAll(text, string(ch), string([]rune(set2)[i]))
		}
	}
	return NewText(text), true, nil
}

func builtinSort(sh *Shell, args []string) (*Result, bool, error) {
	reverse := false
	unique := false
	numeric := false
	var files []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "r") {
				reverse = true
			}
			if strings.Contains(a, "u") {
				unique = true
			}
			if strings.Contains(a, "n") {
				numeric = true
			}
		} else {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		return nil, true, fmt.Errorf("sort: no files specified")
	}
	var allLines []string
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("sort: %w", err)
		}
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		allLines = append(allLines, lines...)
	}
	if numeric {
		sort.SliceStable(allLines, func(i, j int) bool {
			var a, b float64
			fmt.Sscanf(allLines[i], "%f", &a)
			fmt.Sscanf(allLines[j], "%f", &b)
			if reverse {
				return a > b
			}
			return a < b
		})
	} else {
		sort.SliceStable(allLines, func(i, j int) bool {
			if reverse {
				return allLines[i] > allLines[j]
			}
			return allLines[i] < allLines[j]
		})
	}
	if unique {
		seen := map[string]bool{}
		var out []string
		for _, l := range allLines {
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
		allLines = out
	}
	return NewText(strings.Join(allLines, "\n")), true, nil
}

func builtinUniq(sh *Shell, args []string) (*Result, bool, error) {
	count := false
	var files []string
	for _, a := range args {
		if a == "-c" {
			count = true
		} else if !strings.HasPrefix(a, "-") {
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		return nil, true, fmt.Errorf("uniq: no files specified")
	}
	var allLines []string
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("uniq: %w", err)
		}
		allLines = append(allLines, strings.Split(strings.TrimRight(string(data), "\n"), "\n")...)
	}
	if count {
		cols := []string{"count", "line"}
		var rows []Row
		var prev string
		cnt := 0
		for _, l := range allLines {
			if l == prev {
				cnt++
			} else {
				if cnt > 0 {
					rows = append(rows, Row{"count": strconv.Itoa(cnt), "line": prev})
				}
				prev = l
				cnt = 1
			}
		}
		if cnt > 0 {
			rows = append(rows, Row{"count": strconv.Itoa(cnt), "line": prev})
		}
		return NewTable(cols, rows), true, nil
	}
	var out []string
	var prev string
	for _, l := range allLines {
		if l != prev {
			out = append(out, l)
			prev = l
		}
	}
	return NewText(strings.Join(out, "\n")), true, nil
}

func builtinTee(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 2 {
		return nil, true, fmt.Errorf("tee: usage: tee <file> (pipe input required)")
	}
	// In StructSH, tee reads from a file and writes to another (not stdin)
	src := resolvePath(sh.cwd, args[0])
	dst := resolvePath(sh.cwd, args[1])
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, true, fmt.Errorf("tee: %w", err)
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return nil, true, fmt.Errorf("tee: %w", err)
	}
	fmt.Printf("  %stee: wrote %s → %s%s\n", ansiGreen, args[0], args[1], ansiReset)
	return NewText(string(data)), true, nil
}

func builtinSplit(sh *Shell, args []string) (*Result, bool, error) {
	lines := 1000
	prefix := "x"
	var file string
	for i := 0; i < len(args); i++ {
		if args[i] == "-l" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &lines)
			i++
		} else if !strings.HasPrefix(args[i], "-") {
			if file == "" {
				file = args[i]
			} else {
				prefix = args[i]
			}
		}
	}
	if file == "" {
		return nil, true, fmt.Errorf("split: missing file argument")
	}
	p := resolvePath(sh.cwd, file)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, true, fmt.Errorf("split: %w", err)
	}
	allLines := strings.Split(string(data), "\n")
	cols := []string{"chunk", "lines", "file"}
	var rows []Row
	chunk := 0
	for i := 0; i < len(allLines); i += lines {
		end := i + lines
		if end > len(allLines) {
			end = len(allLines)
		}
		name := fmt.Sprintf("%s%02d", prefix, chunk)
		outPath := resolvePath(sh.cwd, name)
		content := strings.Join(allLines[i:end], "\n")
		_ = os.WriteFile(outPath, []byte(content), 0644)
		rows = append(rows, Row{
			"chunk": strconv.Itoa(chunk),
			"lines": strconv.Itoa(end - i),
			"file":  name,
		})
		chunk++
	}
	return NewTable(cols, rows), true, nil
}

func builtinXargs(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 2 {
		return nil, true, fmt.Errorf("xargs: usage: xargs <cmd> <file-with-args>")
	}
	cmdName := args[0]
	file := args[len(args)-1]
	p := resolvePath(sh.cwd, file)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, true, fmt.Errorf("xargs: %w", err)
	}
	items := strings.Fields(string(data))
	var results []string
	for _, item := range items {
		cmdArgs := append(args[1:len(args)-1], item)
		out, err := rawExec(cmdName, cmdArgs, sh.cwd)
		if err != nil {
			results = append(results, fmt.Sprintf("ERROR(%s): %s", item, err.Error()))
		} else {
			results = append(results, out)
		}
	}
	return NewText(strings.Join(results, "\n")), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  PERMISSIONS
// ═════════════════════════════════════════════════════════════════════════════

func builtinChmod(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) < 2 {
		return nil, true, fmt.Errorf("chmod: usage: chmod <mode> <file...>")
	}
	modeStr := args[0]
	files := args[1:]

	// Numeric mode
	var mode os.FileMode
	if _, err := fmt.Sscanf(modeStr, "%o", &mode); err == nil {
		cols := []string{"status", "mode", "file"}
		var rows []Row
		for _, f := range files {
			p := resolvePath(sh.cwd, f)
			err := os.Chmod(p, mode)
			status := fmt.Sprintf("0%o", mode)
			if err != nil {
				status = "error: " + err.Error()
			}
			rows = append(rows, Row{"status": "ok", "mode": status, "file": f})
		}
		return NewTable(cols, rows), true, nil
	}
	// Symbolic mode (u+x, a-w, etc.)
	cols := []string{"status", "file"}
	var rows []Row
	for _, f := range files {
		p := resolvePath(sh.cwd, f)
		info, err := os.Stat(p)
		if err != nil {
			rows = append(rows, Row{"status": "error: " + err.Error(), "file": f})
			continue
		}
		newMode := applySymbolicChmod(info.Mode(), modeStr)
		err = os.Chmod(p, newMode)
		status := "ok"
		if err != nil {
			status = "error: " + err.Error()
		}
		rows = append(rows, Row{"status": status, "file": f})
	}
	return NewTable(cols, rows), true, nil
}

func applySymbolicChmod(mode os.FileMode, expr string) os.FileMode {
	// Very simplified: u+x, a+w, o-r, g=rw etc.
	re := regexp.MustCompile(`([ugoa]*)([+\-=])([rwx]*)`)
	m := re.FindStringSubmatch(expr)
	if m == nil {
		return mode
	}
	who := m[1]
	op := m[2]
	perms := m[3]

	if who == "" || who == "a" {
		who = "ugo"
	}

	var bits os.FileMode
	if strings.Contains(perms, "r") {
		bits |= 0444
	}
	if strings.Contains(perms, "w") {
		bits |= 0222
	}
	if strings.Contains(perms, "x") {
		bits |= 0111
	}

	// Mask to relevant who
	var mask os.FileMode
	if strings.Contains(who, "u") {
		mask |= 0700
	}
	if strings.Contains(who, "g") {
		mask |= 0070
	}
	if strings.Contains(who, "o") {
		mask |= 0007
	}
	bits &= mask

	switch op {
	case "+":
		return mode | bits
	case "-":
		return mode &^ bits
	case "=":
		return (mode &^ mask) | bits
	}
	return mode
}

func builtinChown(sh *Shell, args []string) (*Result, bool, error) {
	// Pure Go cannot change ownership without syscalls; delegate to system chown.
	if len(args) < 2 {
		return nil, true, fmt.Errorf("chown: usage: chown <user[:group]> <file...>")
	}
	out, err := rawExec("chown", args, sh.cwd)
	if err != nil {
		return nil, true, fmt.Errorf("chown: %w", err)
	}
	if out == "" {
		out = "done"
	}
	return NewText(out), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  PROCESS
// ═════════════════════════════════════════════════════════════════════════════

func builtinPS(sh *Shell, args []string) (*Result, bool, error) {
	psArgs := args
	if len(psArgs) == 0 {
		psArgs = []string{"aux"}
	}
	out, err := rawExec("ps", psArgs, "")
	if err != nil {
		return nil, true, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return NewText(out), true, nil
	}
	cols := []string{"pid", "user", "cpu", "mem", "stat", "started", "command"}
	var rows []Row
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) < 11 {
			continue
		}
		rows = append(rows, Row{
			"user":    f[0],
			"pid":     f[1],
			"cpu":     f[2] + "%",
			"mem":     f[3] + "%",
			"stat":    f[7],
			"started": f[8],
			"command": truncStr(strings.Join(f[10:], " "), 60),
		})
	}
	return NewTable(cols, rows), true, nil
}

func builtinKill(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("kill: usage: kill [-SIGNAL] <pid...>")
	}
	signal := "TERM"
	var pids []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			signal = strings.TrimPrefix(a, "-")
		} else {
			pids = append(pids, a)
		}
	}
	cols := []string{"pid", "signal", "status"}
	var rows []Row
	for _, pid := range pids {
		out, err := rawExec("kill", []string{"-" + signal, pid}, "")
		status := "sent"
		if err != nil {
			status = "error: " + err.Error()
		}
		_ = out
		rows = append(rows, Row{"pid": pid, "signal": signal, "status": status})
	}
	return NewTable(cols, rows), true, nil
}

func builtinSleep(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("sleep: usage: sleep <seconds>")
	}
	var secs float64
	if _, err := fmt.Sscanf(args[0], "%f", &secs); err != nil {
		return nil, true, fmt.Errorf("sleep: invalid duration %q", args[0])
	}
	if secs > 60 {
		return nil, true, fmt.Errorf("sleep: max 60 seconds in interactive mode")
	}
	time.Sleep(time.Duration(secs * float64(time.Second)))
	return NewText(""), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  SYSTEM INFO
// ═════════════════════════════════════════════════════════════════════════════

func builtinUname(sh *Shell, args []string) (*Result, bool, error) {
	all := len(args) > 0 && args[0] == "-a"
	hostname, _ := os.Hostname()
	cols := []string{"os", "arch", "hostname"}
	row := Row{
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"hostname": hostname,
	}
	if all {
		cols = append(cols, "cpus", "go_version")
		row["cpus"] = strconv.Itoa(runtime.NumCPU())
		row["go_version"] = runtime.Version()
	}
	return NewTable(cols, []Row{row}), true, nil
}

func builtinUptime(sh *Shell, args []string) (*Result, bool, error) {
	// Delegate to system uptime
	out, err := rawExec("uptime", args, "")
	if err != nil {
		return nil, true, err
	}
	return NewText(strings.TrimSpace(out)), true, nil
}

func builtinDate(sh *Shell, args []string) (*Result, bool, error) {
	format := time.RFC1123
	if len(args) > 0 && strings.HasPrefix(args[0], "+") {
		// Convert strftime-ish to Go format (basic subset)
		f := args[0][1:]
		f = strings.ReplaceAll(f, "%Y", "2006")
		f = strings.ReplaceAll(f, "%m", "01")
		f = strings.ReplaceAll(f, "%d", "02")
		f = strings.ReplaceAll(f, "%H", "15")
		f = strings.ReplaceAll(f, "%M", "04")
		f = strings.ReplaceAll(f, "%S", "05")
		f = strings.ReplaceAll(f, "%A", "Monday")
		f = strings.ReplaceAll(f, "%B", "January")
		f = strings.ReplaceAll(f, "%Z", "MST")
		format = f
	}
	now := time.Now()
	cols := []string{"date", "time", "timezone", "unix"}
	row := Row{
		"date":     now.Format("2006-01-02"),
		"time":     now.Format("15:04:05"),
		"timezone": now.Format("MST"),
		"unix":     strconv.FormatInt(now.Unix(), 10),
	}
	if len(args) > 0 {
		return NewText(now.Format(format)), true, nil
	}
	return NewTable(cols, []Row{row}), true, nil
}

func builtinCal(sh *Shell, args []string) (*Result, bool, error) {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())
	if len(args) >= 2 {
		fmt.Sscanf(args[0], "%d", &month)
		fmt.Sscanf(args[1], "%d", &year)
	} else if len(args) == 1 {
		fmt.Sscanf(args[0], "%d", &year)
		month = 0 // full year
	}

	var sb strings.Builder
	if month > 0 {
		sb.WriteString(renderCalMonth(year, time.Month(month), now))
	} else {
		for m := 1; m <= 12; m++ {
			sb.WriteString(renderCalMonth(year, time.Month(m), now))
			sb.WriteString("\n")
		}
	}
	return NewText(sb.String()), true, nil
}

func renderCalMonth(year int, month time.Month, now time.Time) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("   %s %d\n", month.String(), year))
	sb.WriteString(" Su Mo Tu We Th Fr Sa\n")
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	startDay := int(first.Weekday())
	sb.WriteString(strings.Repeat("   ", startDay))
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	col := startDay
	for d := 1; d <= last; d++ {
		isToday := year == now.Year() && month == now.Month() && d == now.Day()
		if isToday {
			sb.WriteString(fmt.Sprintf("[%2d]", d))
		} else {
			sb.WriteString(fmt.Sprintf(" %2d", d))
		}
		col++
		if col%7 == 0 {
			sb.WriteString("\n")
		}
	}
	if col%7 != 0 {
		sb.WriteString("\n")
	}
	return sb.String()
}

func builtinHostname(sh *Shell, args []string) (*Result, bool, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, true, err
	}
	if len(args) > 0 && args[0] == "-i" {
		addrs, err := net.LookupHost(h)
		if err != nil {
			return NewText(h), true, nil
		}
		cols := []string{"hostname", "ip"}
		var rows []Row
		for _, a := range addrs {
			rows = append(rows, Row{"hostname": h, "ip": a})
		}
		return NewTable(cols, rows), true, nil
	}
	return NewText(h), true, nil
}

func builtinID(sh *Shell, args []string) (*Result, bool, error) {
	out, err := rawExec("id", args, "")
	if err != nil {
		return nil, true, err
	}
	// Parse "uid=1000(user) gid=1000(group) groups=..."
	cols := []string{"field", "value"}
	var rows []Row
	for _, part := range strings.Fields(out) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			rows = append(rows, Row{"field": kv[0], "value": kv[1]})
		}
	}
	if len(rows) == 0 {
		return NewText(out), true, nil
	}
	return NewTable(cols, rows), true, nil
}

func builtinGroups(sh *Shell, args []string) (*Result, bool, error) {
	out, err := rawExec("groups", args, "")
	if err != nil {
		return nil, true, err
	}
	groups := strings.Fields(strings.TrimSpace(out))
	cols := []string{"index", "group"}
	var rows []Row
	for i, g := range groups {
		rows = append(rows, Row{"index": strconv.Itoa(i), "group": g})
	}
	return NewTable(cols, rows), true, nil
}

func builtinWho(sh *Shell, args []string) (*Result, bool, error) {
	out, err := rawExec("who", args, "")
	if err != nil {
		return nil, true, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	cols := []string{"user", "tty", "login_time", "from"}
	var rows []Row
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		from := ""
		if len(f) > 4 {
			from = strings.Trim(strings.Join(f[4:], " "), "()")
		}
		rows = append(rows, Row{
			"user":       f[0],
			"tty":        f[1],
			"login_time": strings.Join(f[2:4], " "),
			"from":       from,
		})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  NETWORK
// ═════════════════════════════════════════════════════════════════════════════

func builtinPing(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("ping: usage: ping [-c N] <host>")
	}
	// Add -c 4 if not specified
	hasCount := false
	for _, a := range args {
		if a == "-c" {
			hasCount = true
		}
	}
	if !hasCount {
		args = append([]string{"-c", "4"}, args...)
	}
	out, err := rawExec("ping", args, "")
	if err != nil {
		return nil, true, fmt.Errorf("ping: %w", err)
	}
	// Parse ping output into structured rows
	lines := strings.Split(strings.TrimSpace(out), "\n")
	cols := []string{"seq", "host", "bytes", "ttl", "time"}
	var rows []Row
	re := regexp.MustCompile(`(\d+) bytes from ([^:]+).*icmp_seq=(\d+).*ttl=(\d+).*time=([\d.]+)`)
	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if m != nil {
			rows = append(rows, Row{
				"seq":   m[3],
				"host":  m[2],
				"bytes": m[1],
				"ttl":   m[4],
				"time":  m[5] + "ms",
			})
		}
	}
	if len(rows) == 0 {
		return NewText(out), true, nil
	}
	// Append summary
	result := NewTable(cols, rows)
	return result, true, nil
}

func builtinCurl(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("curl: usage: curl [-I] [-o file] [-X method] <url>")
	}
	curlArgs := []string{"-s", "-L"}
	var outputFile string
	for i := 0; i < len(args); i++ {
		if args[i] == "-o" && i+1 < len(args) {
			outputFile = resolvePath(sh.cwd, args[i+1])
			curlArgs = append(curlArgs, "-o", outputFile)
			i++
		} else {
			curlArgs = append(curlArgs, args[i])
		}
	}
	out, err := rawExec("curl", curlArgs, "")
	if err != nil {
		return nil, true, fmt.Errorf("curl: %w", err)
	}
	return NewText(out), true, nil
}

func builtinNslookup(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("nslookup: usage: nslookup <hostname>")
	}
	host := args[0]
	addrs, err := net.LookupHost(host)
	if err != nil {
		return nil, true, fmt.Errorf("nslookup: %w", err)
	}
	cnames, _ := net.LookupCNAME(host)
	txts, _ := net.LookupTXT(host)

	cols := []string{"type", "value"}
	var rows []Row
	for _, a := range addrs {
		rows = append(rows, Row{"type": "A/AAAA", "value": a})
	}
	if cnames != "" && cnames != host+"." {
		rows = append(rows, Row{"type": "CNAME", "value": cnames})
	}
	for _, t := range txts {
		rows = append(rows, Row{"type": "TXT", "value": t})
	}
	return NewTable(cols, rows), true, nil
}

func builtinIfconfig(sh *Shell, args []string) (*Result, bool, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, true, err
	}
	cols := []string{"interface", "flags", "mtu", "ipv4", "ipv6", "mac"}
	var rows []Row
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		var ipv4, ipv6 []string
		for _, addr := range addrs {
			s := addr.String()
			if strings.Contains(s, ":") {
				ipv6 = append(ipv6, s)
			} else {
				ipv4 = append(ipv4, s)
			}
		}
		rows = append(rows, Row{
			"interface": iface.Name,
			"flags":     iface.Flags.String(),
			"mtu":       strconv.Itoa(iface.MTU),
			"ipv4":      strings.Join(ipv4, " "),
			"ipv6":      strings.Join(ipv6, " "),
			"mac":       iface.HardwareAddr.String(),
		})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  HASHING
// ═════════════════════════════════════════════════════════════════════════════

func builtinHash(sh *Shell, args []string, algo string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("%ssum: missing file argument", algo)
	}
	cols := []string{"hash", "file"}
	var rows []Row
	for _, f := range args {
		p := resolvePath(sh.cwd, f)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, true, fmt.Errorf("%s: %s: %w", algo, f, err)
		}
		var hash string
		switch algo {
		case "md5":
			h := md5.Sum(data)
			hash = fmt.Sprintf("%x", h)
		case "sha1":
			h := sha1.Sum(data)
			hash = fmt.Sprintf("%x", h)
		case "sha256":
			h := sha256.Sum256(data)
			hash = fmt.Sprintf("%x", h)
		}
		rows = append(rows, Row{"hash": hash, "file": f})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  ARCHIVING (system delegation)
// ═════════════════════════════════════════════════════════════════════════════

func builtinSysDelegate(sh *Shell, command string, args []string) (*Result, bool, error) {
	out, err := rawExec(command, args, sh.cwd)
	if err != nil {
		return nil, true, err
	}
	if out == "" {
		out = "done"
	}
	return NewText(out), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  TEXT GENERATION & ENCODING
// ═════════════════════════════════════════════════════════════════════════════

func builtinEcho(sh *Shell, args []string) (*Result, bool, error) {
	noNewline := false
	interpretEscapes := false
	var parts []string
	for _, a := range args {
		if a == "-n" {
			noNewline = true
		} else if a == "-e" {
			interpretEscapes = true
		} else {
			parts = append(parts, a)
		}
	}
	text := strings.Join(parts, " ")
	if interpretEscapes {
		text = strings.ReplaceAll(text, `\n`, "\n")
		text = strings.ReplaceAll(text, `\t`, "\t")
		text = strings.ReplaceAll(text, `\r`, "\r")
	}
	if noNewline {
		fmt.Print(text)
		return NewText(""), true, nil
	}
	return NewText(text), true, nil
}

func builtinPrintf(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("printf: missing format string")
	}
	fmtStr := args[0]
	fmtStr = strings.ReplaceAll(fmtStr, `\n`, "\n")
	fmtStr = strings.ReplaceAll(fmtStr, `\t`, "\t")

	iargs := make([]interface{}, len(args)-1)
	for i, a := range args[1:] {
		// Try numeric
		if f, err := strconv.ParseFloat(a, 64); err == nil {
			iargs[i] = f
		} else {
			iargs[i] = a
		}
	}
	var out string
	if len(iargs) > 0 {
		out = fmt.Sprintf(fmtStr, iargs...)
	} else {
		out = fmtStr
	}
	return NewText(out), true, nil
}

func builtinYes(sh *Shell, args []string) (*Result, bool, error) {
	word := "y"
	if len(args) > 0 {
		word = strings.Join(args, " ")
	}
	// Return 20 repetitions instead of infinite
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, word)
	}
	return NewText(strings.Join(lines, "\n") + "\n(truncated to 20 lines)"), true, nil
}

func builtinSeq(sh *Shell, args []string) (*Result, bool, error) {
	var first, step, last float64
	switch len(args) {
	case 1:
		first = 1
		step = 1
		fmt.Sscanf(args[0], "%f", &last)
	case 2:
		fmt.Sscanf(args[0], "%f", &first)
		step = 1
		fmt.Sscanf(args[1], "%f", &last)
	case 3:
		fmt.Sscanf(args[0], "%f", &first)
		fmt.Sscanf(args[1], "%f", &step)
		fmt.Sscanf(args[2], "%f", &last)
	default:
		return nil, true, fmt.Errorf("seq: usage: seq [first [step]] last")
	}
	if step == 0 {
		return nil, true, fmt.Errorf("seq: step cannot be zero")
	}
	cols := []string{"n", "value"}
	var rows []Row
	i := 0
	for v := first; (step > 0 && v <= last) || (step < 0 && v >= last); v += step {
		rows = append(rows, Row{"n": strconv.Itoa(i + 1), "value": strconv.FormatFloat(v, 'f', -1, 64)})
		i++
		if i > 10000 {
			break
		}
	}
	return NewTable(cols, rows), true, nil
}

func builtinBase64(sh *Shell, args []string) (*Result, bool, error) {
	decode := false
	var file string
	for _, a := range args {
		if a == "-d" || a == "--decode" {
			decode = true
		} else {
			file = a
		}
	}
	if file == "" {
		return nil, true, fmt.Errorf("base64: missing file argument")
	}
	p := resolvePath(sh.cwd, file)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, true, fmt.Errorf("base64: %w", err)
	}
	if decode {
		// Decode base64 from file
		out, err := rawExec("base64", []string{"-d", p}, "")
		if err != nil {
			return nil, true, fmt.Errorf("base64: %w", err)
		}
		return NewText(out), true, nil
	}
	encoded := encodeBase64(data)
	return NewText(encoded), true, nil
}

func encodeBase64(data []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var sb strings.Builder
	for i := 0; i < len(data); i += 3 {
		b0 := data[i]
		var b1, b2 byte
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		sb.WriteByte(chars[b0>>2])
		sb.WriteByte(chars[((b0&0x3)<<4)|(b1>>4)])
		if i+1 < len(data) {
			sb.WriteByte(chars[((b1&0xf)<<2)|(b2>>6)])
		} else {
			sb.WriteByte('=')
		}
		if i+2 < len(data) {
			sb.WriteByte(chars[b2&0x3f])
		} else {
			sb.WriteByte('=')
		}
	}
	// Wrap at 76 chars
	raw := sb.String()
	var wrapped strings.Builder
	for i := 0; i < len(raw); i += 76 {
		end := i + 76
		if end > len(raw) {
			end = len(raw)
		}
		wrapped.WriteString(raw[i:end])
		wrapped.WriteString("\n")
	}
	return wrapped.String()
}

func builtinRev(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("rev: missing file argument")
	}
	var out []string
	for _, f := range args {
		data, err := os.ReadFile(resolvePath(sh.cwd, f))
		if err != nil {
			return nil, true, fmt.Errorf("rev: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			runes := []rune(line)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			out = append(out, string(runes))
		}
	}
	return NewText(strings.Join(out, "\n")), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  SHELL VARIABLES & ENVIRONMENT
// ═════════════════════════════════════════════════════════════════════════════

func builtinSet(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return builtinVars(sh)
	}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			return nil, true, fmt.Errorf("set: invalid format — use NAME=VALUE")
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
	// Collect and sort
	var names []string
	for k := range sh.vars {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		rows = append(rows, Row{"name": k, "value": sh.vars[k]})
	}
	return NewTable(cols, rows), true, nil
}

func builtinExport(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		// Show exported vars
		return builtinEnv(sh, nil)
	}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 2 {
			sh.vars[parts[0]] = parts[1]
			if err := os.Setenv(parts[0], parts[1]); err != nil {
				return nil, true, fmt.Errorf("export: %w", err)
			}
		} else {
			// Export existing var
			if v, ok := sh.vars[a]; ok {
				_ = os.Setenv(a, v)
			}
		}
	}
	return NewText(""), true, nil
}

func builtinEnv(sh *Shell, args []string) (*Result, bool, error) {
	cols := []string{"key", "value"}
	var rows []Row
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			rows = append(rows, Row{"key": parts[0], "value": parts[1]})
		}
	}
	return NewTable(cols, rows), true, nil
}

func builtinPrintenv(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return builtinEnv(sh, nil)
	}
	cols := []string{"key", "value"}
	var rows []Row
	for _, k := range args {
		v := os.Getenv(k)
		rows = append(rows, Row{"key": k, "value": v})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  ALIAS
// ═════════════════════════════════════════════════════════════════════════════

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
	var names []string
	for n := range sh.aliases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		a := sh.aliases[n]
		rows = append(rows, Row{
			"name":       a.Name,
			"expands_to": a.Expand,
			"created":    a.Created.Format("15:04:05"),
		})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  IDENTIFICATION
// ═════════════════════════════════════════════════════════════════════════════

func builtinWhich(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("which: missing argument")
	}
	cols := []string{"command", "path", "type"}
	var rows []Row
	for _, cmd := range args {
		// Check aliases first
		if a, ok := sh.aliases[cmd]; ok {
			rows = append(rows, Row{"command": cmd, "path": a.Expand, "type": "alias"})
			continue
		}
		// Check builtins
		if isBuiltin(cmd) {
			rows = append(rows, Row{"command": cmd, "path": "(built-in)", "type": "builtin"})
			continue
		}
		// Check PATH
		p := findInPath(cmd)
		if p != "" {
			rows = append(rows, Row{"command": cmd, "path": p, "type": "external"})
		} else {
			rows = append(rows, Row{"command": cmd, "path": "(not found)", "type": "unknown"})
		}
	}
	return NewTable(cols, rows), true, nil
}

func builtinType(sh *Shell, args []string) (*Result, bool, error) {
	return builtinWhich(sh, args)
}

func isBuiltin(cmd string) bool {
	builtins := []string{
		"cd", "pwd", "pushd", "popd", "dirs", "ls", "ll", "la", "tree", "du", "df",
		"cat", "head", "tail", "touch", "mkdir", "rmdir", "rm", "cp", "mv", "ln",
		"wc", "stat", "file", "find", "diff", "grep", "sed", "awk", "cut", "tr",
		"sort", "uniq", "tee", "split", "xargs", "chmod", "chown",
		"ps", "kill", "sleep", "jobs", "uname", "uptime", "date", "cal",
		"hostname", "whoami", "id", "groups", "w", "who",
		"ping", "curl", "wget", "nslookup", "dig", "ifconfig", "ip",
		"md5sum", "sha1sum", "sha256sum", "md5", "sha1", "sha256",
		"tar", "gzip", "gunzip", "zip", "unzip",
		"echo", "printf", "yes", "seq", "base64", "rev",
		"set", "unset", "vars", "export", "env", "printenv",
		"alias", "unalias", "aliases", "which", "type",
		"bc", "factor", "random",
		"box", "history", "clear", "help", "man", "true", "false",
		"exit", "quit", "source", "watch",
	}
	for _, b := range builtins {
		if b == cmd {
			return true
		}
	}
	return false
}

// ═════════════════════════════════════════════════════════════════════════════
//  NUMERIC & MISC UTILS
// ═════════════════════════════════════════════════════════════════════════════

func builtinBC(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("bc: usage: bc <expression>")
	}
	expr := strings.Join(args, " ")
	// Try to use system bc
	out, err := rawExec("bash", []string{"-c", fmt.Sprintf("echo '%s' | bc -l", expr)}, "")
	if err != nil {
		// Fallback: very basic Go expression eval
		result := evalSimpleExpr(expr)
		return NewText(result), true, nil
	}
	return NewText(strings.TrimSpace(out)), true, nil
}

// evalSimpleExpr handles very basic math: +, -, *, /
func evalSimpleExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	// Try parsing a simple "a op b" expression
	for _, op := range []string{"+", "-", "*", "/"} {
		parts := strings.SplitN(expr, op, 2)
		if len(parts) == 2 {
			var a, b float64
			if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &a); err != nil {
				continue
			}
			if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &b); err != nil {
				continue
			}
			var result float64
			switch op {
			case "+":
				result = a + b
			case "-":
				result = a - b
			case "*":
				result = a * b
			case "/":
				if b == 0 {
					return "division by zero"
				}
				result = a / b
			}
			return strconv.FormatFloat(result, 'f', -1, 64)
		}
	}
	return "cannot evaluate: " + expr
}

func builtinFactor(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("factor: usage: factor <number>")
	}
	cols := []string{"number", "factors"}
	var rows []Row
	for _, a := range args {
		var n int64
		if _, err := fmt.Sscanf(a, "%d", &n); err != nil {
			rows = append(rows, Row{"number": a, "factors": "invalid"})
			continue
		}
		factors := primeFactors(n)
		parts := make([]string, len(factors))
		for i, f := range factors {
			parts[i] = strconv.FormatInt(f, 10)
		}
		rows = append(rows, Row{"number": a, "factors": strings.Join(parts, " × ")})
	}
	return NewTable(cols, rows), true, nil
}

func primeFactors(n int64) []int64 {
	var factors []int64
	for n%2 == 0 {
		factors = append(factors, 2)
		n /= 2
	}
	for i := int64(3); i*i <= n; i += 2 {
		for n%i == 0 {
			factors = append(factors, i)
			n /= i
		}
	}
	if n > 2 {
		factors = append(factors, n)
	}
	return factors
}

func builtinRandom(sh *Shell, args []string) (*Result, bool, error) {
	min := 0
	max := 100
	count := 1
	if len(args) >= 1 {
		fmt.Sscanf(args[0], "%d", &max)
	}
	if len(args) >= 2 {
		min = max
		fmt.Sscanf(args[1], "%d", &max)
	}
	if len(args) >= 3 {
		fmt.Sscanf(args[2], "%d", &count)
	}
	if count > 1000 {
		count = 1000
	}
	cols := []string{"n", "value"}
	var rows []Row
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < count; i++ {
		v := r.Intn(max-min+1) + min
		rows = append(rows, Row{"n": strconv.Itoa(i + 1), "value": strconv.Itoa(v)})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  HISTORY
// ═════════════════════════════════════════════════════════════════════════════

func builtinHistory(sh *Shell, args []string) (*Result, bool, error) {
	n := len(sh.history)
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &n)
	}
	start := len(sh.history) - n
	if start < 0 {
		start = 0
	}
	cols := []string{"n", "command", "time", "exit"}
	var rows []Row
	for i, entry := range sh.history[start:] {
		exitStr := "0"
		if entry.ExitCode != 0 {
			exitStr = strconv.Itoa(entry.ExitCode)
		}
		rows = append(rows, Row{
			"n":       strconv.Itoa(start + i + 1),
			"command": entry.Raw,
			"time":    entry.At.Format("15:04:05"),
			"exit":    exitStr,
		})
	}
	return NewTable(cols, rows), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  BOX
// ═════════════════════════════════════════════════════════════════════════════

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
			fmt.Println(c(ansiGrey, "\n  Box is empty. Store with: cmd #=name  or  cmd #="))
			return NewText(""), true, nil
		}
		sh.printBoxList(entries)
		return NewText(""), true, nil
	}
}

// ═════════════════════════════════════════════════════════════════════════════
//  MISC SHELL COMMANDS
// ═════════════════════════════════════════════════════════════════════════════

func builtinMan(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("man: what manual page do you want?")
	}
	cmd := args[0]
	// Try system man first with -f (whatis)
	out, err := rawExec("man", []string{"-f", cmd}, "")
	if err == nil && out != "" {
		return NewText(out), true, nil
	}
	// Fallback: check if it's our built-in
	if isBuiltin(cmd) {
		return NewText(fmt.Sprintf("%s: StructSH built-in command. Use 'help' for full documentation.", cmd)), true, nil
	}
	return NewText(fmt.Sprintf("No manual entry for %s", cmd)), true, nil
}

func builtinSource(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("source: missing file argument")
	}
	p := resolvePath(sh.cwd, args[0])
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, true, fmt.Errorf("source: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	var results []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		code := sh.execLine(line)
		if code != 0 {
			results = append(results, fmt.Sprintf("line error: %s", line))
		}
	}
	if len(results) > 0 {
		return NewText(strings.Join(results, "\n")), true, nil
	}
	return NewText(fmt.Sprintf("sourced %d lines from %s", len(lines), args[0])), true, nil
}

func builtinWatch(sh *Shell, args []string) (*Result, bool, error) {
	if len(args) == 0 {
		return nil, true, fmt.Errorf("watch: usage: watch [-n seconds] <command>")
	}
	interval := 2.0
	var cmdArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%f", &interval)
			i++
		} else {
			cmdArgs = args[i:]
			break
		}
	}
	if len(cmdArgs) == 0 {
		return nil, true, fmt.Errorf("watch: missing command")
	}
	if interval > 10 {
		interval = 10
	}
	iterations := 3 // Run 3 times in built-in mode
	fmt.Printf("  %swatch: running %q every %.0fs (%d times)%s\n\n",
		ansiYellow, strings.Join(cmdArgs, " "), interval, iterations, ansiReset)
	for i := 0; i < iterations; i++ {
		fmt.Printf("  %s─── iteration %d ───%s\n", ansiGrey, i+1, ansiReset)
		sh.execLine(strings.Join(cmdArgs, " "))
		if i < iterations-1 {
			time.Sleep(time.Duration(interval * float64(time.Second)))
		}
	}
	return NewText(""), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  HELP
// ═════════════════════════════════════════════════════════════════════════════

func builtinHelp() (*Result, bool, error) {
	help := "\n" + sectionHeader("StructSH — Structured Shell v0.2.0") + `
  Every command output is a table. Chain transforms with | pipes.
  Store any result in the Box with #=.

` + c(ansiBold+ansiCyan, "  ── NAVIGATION ────────────────────────────────────────────") + `
  cd [dir|-]          change directory (- for previous)
  pwd                 print working directory
  pushd <dir>         push dir onto stack and cd
  popd                pop stack and cd back
  dirs                show directory stack

` + c(ansiBold+ansiCyan, "  ── LISTING ────────────────────────────────────────────────") + `
  ls [-la] [dir]      list files as table
  ll [dir]            ls -l shorthand
  la [dir]            ls -la shorthand
  tree [-L N] [dir]   visual directory tree
  du [-s] [dir]       disk usage per entry
  df                  filesystem usage

` + c(ansiBold+ansiCyan, "  ── FILE OPERATIONS ────────────────────────────────────────") + `
  cat [-n] <file>     show file (optional line numbers)
  head [-n N] <file>  first N lines
  tail [-n N] <file>  last N lines
  touch <file...>     create/update files (shows table)
  mkdir [-p] <dir>    create directory
  rmdir <dir>         remove empty directory
  rm [-rf] <file>     remove files/dirs
  cp [-rv] <src> <dst> copy file or dir
  mv [-v] <src> <dst> move/rename
  ln [-s] <tgt> <lnk> create hard/soft link

` + c(ansiBold+ansiCyan, "  ── INSPECTION ─────────────────────────────────────────────") + `
  wc [-lwc] <file>    word/line/byte count (table)
  stat <file...>      file metadata as table
  file <file...>      detect file type
  find [dir] [-name pattern] [-type f|d] [-maxdepth N] [-newer file]
  diff <file1> <file2> line-by-line diff as table

` + c(ansiBold+ansiCyan, "  ── TEXT PROCESSING ────────────────────────────────────────") + `
  grep [-invr] <pat> <file>   regex search → table
  sed 's/old/new/[g]' <file>  substitution
  sed '/pat/d' <file>         delete matching lines
  awk [-F sep] '{print $N}' <file>  field extract → table
  cut -f N[-M] [-d sep] <file>      cut columns → table
  tr <set1> <set2> <file>     transliterate characters
  sort [-rnu] <file>          sort lines
  uniq [-c] <file>            remove/count duplicates
  split [-l N] <file> [prefix] split into chunks
  tee <src> <dst>             copy and show file
  xargs <cmd> <file>          run cmd per line in file

` + c(ansiBold+ansiCyan, "  ── PERMISSIONS ────────────────────────────────────────────") + `
  chmod <mode> <file...>   set permissions (numeric or u+x style)
  chown <user> <file...>   change ownership (delegates to system)

` + c(ansiBold+ansiCyan, "  ── PROCESS ─────────────────────────────────────────────────") + `
  ps [aux]              list processes (table)
  kill [-SIG] <pid...>  send signal
  sleep <seconds>       pause (max 60s)
  jobs                  show background jobs

` + c(ansiBold+ansiCyan, "  ── SYSTEM INFO ─────────────────────────────────────────────") + `
  uname [-a]            OS and arch info
  uptime                system uptime
  date [+format]        current date/time (table or formatted)
  cal [month] [year]    calendar
  hostname [-i]         hostname and IP
  whoami                current user
  id                    uid/gid/groups table
  groups                group membership
  who / w               logged-in users

` + c(ansiBold+ansiCyan, "  ── NETWORK ─────────────────────────────────────────────────") + `
  ping [-c N] <host>    ping (structured output)
  curl [-o file] <url>  HTTP request
  nslookup <host>       DNS lookup → table
  ifconfig / ip         network interfaces → table

` + c(ansiBold+ansiCyan, "  ── HASHING ─────────────────────────────────────────────────") + `
  md5sum / sha1sum / sha256sum <file...>   hash files → table

` + c(ansiBold+ansiCyan, "  ── ARCHIVING (system) ──────────────────────────────────────") + `
  tar, gzip, gunzip, zip, unzip  (delegated to system)

` + c(ansiBold+ansiCyan, "  ── TEXT GENERATION ────────────────────────────────────────") + `
  echo [-ne] <text>     print text
  printf <fmt> [args]   formatted print
  yes [word]            repeat word (20 lines)
  seq [first [step]] last   number sequence → table
  base64 [-d] <file>    encode/decode base64
  rev <file>            reverse each line

` + c(ansiBold+ansiCyan, "  ── VARIABLES & ENVIRONMENT ────────────────────────────────") + `
  set NAME=VAL          set session variable
  unset NAME            remove variable
  vars                  list session variables
  export NAME=VAL       set and export to OS env
  env / printenv        show environment variables

` + c(ansiBold+ansiCyan, "  ── IDENTIFICATION ──────────────────────────────────────────") + `
  which <cmd>           find command (alias/builtin/path)
  type <cmd>            same as which
  alias name=cmd        define alias
  unalias name          remove alias
  aliases               list all aliases

` + c(ansiBold+ansiCyan, "  ── NUMERIC & MISC ──────────────────────────────────────────") + `
  bc <expr>             calculate expression
  factor <n>            prime factorization
  random [max [min [count]]]  random numbers → table

` + c(ansiBold+ansiCyan, "  ── PIPE OPERATORS ─────────────────────────────────────────") + `
  | select col1,col2    keep columns
  | where col=val       filter rows  (= != > < >= <= ~)
  | grep text           search all columns
  | sort col [asc|desc] sort rows
  | limit N             first N rows
  | skip N              skip N rows
  | count               count rows
  | unique [col]        deduplicate
  | reverse             flip order
  | fmt json|csv|tsv    reformat output
  | add col=value       add a column
  | rename old=new      rename a column

` + c(ansiBold+ansiCyan, "  ── BOX STORAGE ────────────────────────────────────────────") + `
  cmd #=           auto-store result
  cmd #=key        store as named key
  box              list all entries
  box get <key>    retrieve entry
  box rm <key>     remove entry
  box rename o n   rename entry
  box tag k tag    add tag
  box search q     search
  box export f     export JSON
  box import f     import JSON
  box clear        wipe all

` + c(ansiBold+ansiCyan, "  ── SESSION ─────────────────────────────────────────────────") + `
  history [N]      last N commands (table)
  source <file>    run script file
  watch [-n s] cmd run command repeatedly
  man <cmd>        manual page / description
  clear            clear screen
  help             this help
  true / false     exit 0 / exit 1
  exit / quit      leave StructSH
`
	return NewText(help), true, nil
}

// ═════════════════════════════════════════════════════════════════════════════
//  HELPERS
// ═════════════════════════════════════════════════════════════════════════════

func resolvePath(cwd, p string) string {
	if p == "" {
		return cwd
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if p == "~" {
		return homeDir()
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
	for _, dir := range strings.Split(os.Getenv("PATH"), ":") {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func osUsername() (string, error) {
	out, err := exec.Command("whoami").Output()
	if err != nil {
		return os.Getenv("USER"), err
	}
	return strings.TrimSpace(string(out)), nil
}

