package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─────────────────────────────────────────────
//  Command executor: runs real OS commands
//  and converts output into structured Results.
// ─────────────────────────────────────────────

// RunExternal executes an OS command and returns a structured Result.
// Known commands get dedicated structured parsers.
// Unknown commands fall back to auto-detection then raw text.
func RunExternal(command string, args []string, cwd string) (*Result, error) {
	switch command {
	case "ls", "dir":
		return execLS(args, cwd)
	case "ps":
		return execPS(args)
	case "env":
		return execEnv()
	case "df":
		return execDF(args)
	case "who", "w":
		return execWho(args)
	case "lsof":
		return execGeneric(command, args, cwd, true)
	case "netstat", "ss":
		return execGeneric(command, args, cwd, true)
	case "top", "htop":
		return execGeneric(command, append([]string{"-b", "-n1"}, args...), cwd, true)
	default:
		return execGeneric(command, args, cwd, false)
	}
}

// ── ls ───────────────────────────────────────

func execLS(args []string, cwd string) (*Result, error) {
	long := false
	all := false
	dir := cwd

	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if strings.Contains(a, "l") {
				long = true
			}
			if strings.Contains(a, "a") {
				all = true
			}
		} else {
			if filepath.IsAbs(a) {
				dir = a
			} else {
				dir = filepath.Join(cwd, a)
			}
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("ls: cannot open '%s': %w", dir, err)
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
			rows = append(rows, Row{
				"perms":    info.Mode().String(),
				"size":     fmtBytes(info.Size()),
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

	return NewTable(cols, rows), nil
}

// ── ps ───────────────────────────────────────

func execPS(args []string) (*Result, error) {
	psArgs := args
	if len(psArgs) == 0 {
		psArgs = []string{"aux"}
	}
	out, err := rawExec("ps", psArgs, "")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return NewText(out), nil
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
	return NewTable(cols, rows), nil
}

// ── env ──────────────────────────────────────

func execEnv() (*Result, error) {
	cols := []string{"key", "value"}
	var rows []Row
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			rows = append(rows, Row{"key": parts[0], "value": parts[1]})
		}
	}
	return NewTable(cols, rows), nil
}

// ── df ───────────────────────────────────────

func execDF(args []string) (*Result, error) {
	dfArgs := args
	if len(dfArgs) == 0 {
		dfArgs = []string{"-h"}
	}
	out, err := rawExec("df", dfArgs, "")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return NewText(out), nil
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
	return NewTable(cols, rows), nil
}

// ── who ──────────────────────────────────────

func execWho(args []string) (*Result, error) {
	out, err := rawExec("who", args, "")
	if err != nil {
		return nil, err
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
			from = strings.Join(f[4:], " ")
			from = strings.Trim(from, "()")
		}
		rows = append(rows, Row{
			"user":       f[0],
			"tty":        f[1],
			"login_time": strings.Join(f[2:4], " "),
			"from":       from,
		})
	}
	return NewTable(cols, rows), nil
}

// ── generic ──────────────────────────────────

func execGeneric(command string, args []string, cwd string, forceTable bool) (*Result, error) {
	out, err := rawExec(command, args, cwd)
	if err != nil {
		return nil, err
	}
	if forceTable || autoDetectTable(out) {
		if r := autoParseTable(out); r != nil {
			return r, nil
		}
	}
	return NewText(out), nil
}

// rawExec runs a command and captures combined stdout+stderr.
func rawExec(command string, args []string, cwd string) (string, error) {
	cmd := exec.Command(command, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	text := strings.TrimRight(string(out), "\n")
	if err != nil {
		if len(out) > 0 {
			return "", fmt.Errorf("%s", text)
		}
		return "", fmt.Errorf("%s: %w", command, err)
	}
	return text, nil
}

// autoDetectTable returns true if the output looks like a tabular header+rows.
func autoDetectTable(s string) bool {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 {
		return false
	}
	header := lines[0]
	words := strings.Fields(header)
	if len(words) < 2 {
		return false
	}
	upper := 0
	for _, w := range words {
		if w == strings.ToUpper(w) {
			upper++
		}
	}
	// Majority uppercase = likely a table header
	return upper >= len(words)/2+1
}

// autoParseTable parses whitespace-aligned columnar text into a Result.
func autoParseTable(s string) *Result {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 {
		return nil
	}
	cols, starts := detectColumns(lines[0])
	if len(cols) < 2 {
		return nil
	}
	var rows []Row
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		rows = append(rows, extractCells(line, cols, starts))
	}
	// lowercase col names
	low := make([]string, len(cols))
	for i, col := range cols {
		low[i] = strings.ToLower(col)
	}
	return NewTable(low, rows)
}

// detectColumns finds column names and their start positions from a header line.
func detectColumns(header string) ([]string, []int) {
	var cols []string
	var starts []int
	inWord := false
	start := 0
	for i, ch := range header {
		if ch != ' ' && !inWord {
			inWord = true
			start = i
		} else if ch == ' ' && inWord {
			rest := header[i:]
			if strings.TrimLeft(rest, " ") != "" {
				cols = append(cols, strings.TrimSpace(header[start:i]))
				starts = append(starts, start)
				inWord = false
			}
		}
	}
	if inWord {
		cols = append(cols, strings.TrimSpace(header[start:]))
		starts = append(starts, start)
	}
	return cols, starts
}

// extractCells maps a data line into a Row using column start positions.
func extractCells(line string, cols []string, starts []int) Row {
	row := make(Row, len(cols))
	for i, col := range cols {
		s := starts[i]
		var val string
		if s >= len(line) {
			val = ""
		} else if i == len(cols)-1 {
			val = strings.TrimSpace(line[s:])
		} else {
			end := starts[i+1]
			if end > len(line) {
				end = len(line)
			}
			val = strings.TrimSpace(line[s:end])
		}
		row[strings.ToLower(col)] = val
	}
	return row
}

// fmtBytes formats a byte count as human-readable.
func fmtBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// truncStr truncates a string to n characters, appending "…" if needed.
func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
