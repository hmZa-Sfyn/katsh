package main

import (
	"os"
	"os/exec"
	"strings"
)

// ─────────────────────────────────────────────
//  Executor: runs external OS commands and
//  auto-parses output into structured Results.
//
//  Known commands (ls, ps, env, df, who, etc.)
//  are handled as built-ins in builtins.go.
//  This file handles truly external commands
//  and provides the raw execution plumbing.
// ─────────────────────────────────────────────

// RunExternal executes an OS command and returns a structured Result.
func RunExternal(command string, args []string, cwd string) (*Result, error) {
	out, err := rawExec(command, args, cwd)
	if err != nil {
		return nil, err
	}
	if autoDetectTable(out) {
		if r := autoParseTable(out); r != nil {
			return r, nil
		}
	}
	return NewText(out), nil
}

// rawExec runs a command, captures combined stdout+stderr.
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
			return "", errExecFailed(command, text, command+" "+strings.Join(args, " "))
		}
		return "", errExecFailed(command, err.Error(), command+" "+strings.Join(args, " "))
	}
	return text, nil
}

// autoDetectTable heuristic: first line has majority UPPERCASE tokens.
func autoDetectTable(s string) bool {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) < 2 {
		return false
	}
	words := strings.Fields(lines[0])
	if len(words) < 2 {
		return false
	}
	upper := 0
	for _, w := range words {
		if w == strings.ToUpper(w) && len(w) > 1 {
			upper++
		}
	}
	return upper >= len(words)/2+1
}

// autoParseTable parses whitespace-aligned columnar text.
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
	low := make([]string, len(cols))
	for i, c := range cols {
		low[i] = strings.ToLower(c)
	}
	return NewTable(low, rows)
}

// detectColumns finds column names and start positions from a header line.
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

// extractCells maps one data line to a Row using column start positions.
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
