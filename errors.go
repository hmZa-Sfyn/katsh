package main

import (
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Rich error reporting with location, hints, and fix suggestions
// ─────────────────────────────────────────────────────────────────────────────

// ShellError is a structured error with context, hint, and fix suggestion.
type ShellError struct {
	Code    string // e.g. "E001"
	Kind    string // e.g. "SyntaxError", "TypeError", "CommandNotFound"
	Message string // short message
	Detail  string // longer explanation
	Source  string // the line of source that caused it
	Col     int    // column offset (0-based, -1 = unknown)
	Hint    string // what to check
	Fix     string // suggested fix / example
	Trace   []TraceFrame
}

type TraceFrame struct {
	At  string // e.g. "line 3 in func bogo_sort"
	Src string // source snippet
}

func (e *ShellError) Error() string { return e.Message }

// PrintError renders a rich, coloured error block to stdout.
func PrintError(err *ShellError) {
	if err == nil {
		return
	}

	// ── Header ──────────────────────────────────────────────────────────────
	fmt.Printf("\n  %s%s[%s] %s%s\n",
		ansiBold+ansiRed, err.Kind, err.Code, err.Message, ansiReset)

	// ── Source + caret ───────────────────────────────────────────────────────
	if err.Source != "" {
		fmt.Printf("  %s│%s %s%s%s\n", ansiGrey, ansiReset, ansiWhite, err.Source, ansiReset)
		if err.Col >= 0 {
			pad := strings.Repeat(" ", err.Col+4)
			fmt.Printf("  %s│%s %s%s^── here%s\n", ansiGrey, ansiReset, pad, ansiRed, ansiReset)
		}
	}

	// ── Detail ───────────────────────────────────────────────────────────────
	if err.Detail != "" {
		fmt.Printf("  %s│%s %s%s%s\n", ansiGrey, ansiReset, ansiDim+ansiWhite, err.Detail, ansiReset)
	}

	// ── Stack trace ──────────────────────────────────────────────────────────
	if len(err.Trace) > 0 {
		fmt.Printf("  %s│  Traceback:%s\n", ansiGrey, ansiReset)
		for i, f := range err.Trace {
			fmt.Printf("  %s│  %s%d: %s%s  %s%s%s\n",
				ansiGrey, ansiDim, i+1, ansiCyan, f.At, ansiGrey, f.Src, ansiReset)
		}
	}

	// ── Hint ─────────────────────────────────────────────────────────────────
	if err.Hint != "" {
		fmt.Printf("  %s╰─ 💡 hint:%s %s\n", ansiYellow, ansiReset, err.Hint)
	}

	// ── Fix ──────────────────────────────────────────────────────────────────
	if err.Fix != "" {
		fmt.Printf("  %s   fix :%s  %s%s%s\n\n",
			ansiGreen, ansiReset, ansiDarkCyan, err.Fix, ansiReset)
	} else {
		fmt.Println()
	}
}

// ── Constructor helpers ──────────────────────────────────────────────────────

func errUnknownCmd(cmd, src string) *ShellError {
	similar := findSimilarCmd(cmd)
	fix := ""
	hint := fmt.Sprintf("%q is not a built-in or executable in $PATH", cmd)
	if similar != "" {
		fix = fmt.Sprintf("did you mean: %s", similar)
	}
	return &ShellError{
		Code:    "E001",
		Kind:    "CommandNotFound",
		Message: fmt.Sprintf("command not found: %q", cmd),
		Source:  src,
		Col:     0,
		Hint:    hint,
		Fix:     fix,
	}
}

func errSyntax(msg, src string, col int) *ShellError {
	return &ShellError{
		Code:    "E002",
		Kind:    "SyntaxError",
		Message: msg,
		Source:  src,
		Col:     col,
		Hint:    "Check for missing colons, brackets, or mismatched quotes",
		Fix:     "",
	}
}

func errType(msg, src string) *ShellError {
	return &ShellError{
		Code:    "E003",
		Kind:    "TypeError",
		Message: msg,
		Source:  src,
		Col:     -1,
	}
}

func errRuntime(msg, src string, trace []TraceFrame) *ShellError {
	return &ShellError{
		Code:    "E004",
		Kind:    "RuntimeError",
		Message: msg,
		Source:  src,
		Col:     -1,
		Trace:   trace,
		Hint:    "Use 'vars' to inspect current variable values",
	}
}

func errUndefined(varName, src string) *ShellError {
	return &ShellError{
		Code:    "E005",
		Kind:    "UndefinedVariable",
		Message: fmt.Sprintf("variable %q is not defined", varName),
		Source:  src,
		Col:     strings.Index(src, varName),
		Hint:    "Declare it first with:  " + varName + " = <value>",
		Fix:     varName + " = \"\"",
	}
}

func errDivZero(src string) *ShellError {
	return &ShellError{
		Code:    "E006",
		Kind:    "DivisionByZero",
		Message: "division by zero",
		Source:  src,
		Col:     strings.Index(src, "/"),
		Hint:    "Check the denominator before dividing",
		Fix:     "if denom != 0: result = num / denom",
	}
}

func errArgCount(funcName string, want, got int, src string) *ShellError {
	return &ShellError{
		Code:    "E007",
		Kind:    "ArgumentError",
		Message: fmt.Sprintf("%s() expects %d argument(s), got %d", funcName, want, got),
		Source:  src,
		Col:     -1,
		Hint:    fmt.Sprintf("func %s takes %d param(s)", funcName, want),
	}
}

func errSimple(msg string) *ShellError {
	return &ShellError{
		Code:    "E000",
		Kind:    "Error",
		Message: msg,
		Col:     -1,
	}
}

// wrapErr wraps a plain Go error as a ShellError for nice display.
func wrapErr(err error, src string) *ShellError {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Detect common patterns
	if strings.Contains(msg, "no such file") {
		return &ShellError{
			Code:    "E010",
			Kind:    "FileNotFound",
			Message: msg,
			Source:  src,
			Col:     -1,
			Hint:    "Check the path exists with: stat <path>",
			Fix:     "ls " + extractPath(msg),
		}
	}
	if strings.Contains(msg, "permission denied") {
		return &ShellError{
			Code:    "E011",
			Kind:    "PermissionDenied",
			Message: msg,
			Source:  src,
			Col:     -1,
			Hint:    "You may need elevated privileges",
			Fix:     "chmod +r <file>   or   sudo <command>",
		}
	}
	if strings.Contains(msg, "not found") {
		// Command not found from exec
		parts := strings.SplitN(msg, ":", 2)
		cmd := strings.TrimSpace(parts[0])
		return errUnknownCmd(cmd, src)
	}
	return &ShellError{
		Code:    "E000",
		Kind:    "Error",
		Message: msg,
		Source:  src,
		Col:     -1,
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// findSimilarCmd finds the closest builtin name using simple edit-distance.
func findSimilarCmd(cmd string) string {
	best := ""
	bestDist := 4 // only suggest if very close
	for _, b := range allBuiltinNames() {
		d := editDistance(cmd, b)
		if d < bestDist {
			bestDist = d
			best = b
		}
	}
	return best
}

func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min3(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[la][lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func extractPath(msg string) string {
	// Try to get path from error message like "open /foo/bar: no such file"
	parts := strings.Fields(msg)
	for _, p := range parts {
		if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./") {
			return strings.TrimSuffix(p, ":")
		}
	}
	return ""
}

// ── Pipe error constructors ─────────────────────────────────────────────────

func errUnknownPipe(op, src string) *ShellError {
	validPipes := []string{
		"select/cols", "where/filter", "grep/search", "sort/orderby",
		"limit/head/top", "skip/offset/tail", "count", "unique/distinct",
		"reverse", "fmt/format", "add/addcol", "rename/renamecol",
	}
	return &ShellError{
		Code:    "E012",
		Kind:    "UnknownPipe",
		Message: fmt.Sprintf("unknown pipe operator: %q", op),
		Source:  src,
		Col:     strings.Index(src, op),
		Hint:    fmt.Sprintf("Available pipes: %s", strings.Join(validPipes, ", ")),
		Fix:     "select col1,col2  |  where col=value  |  grep pattern  |  sort col asc",
	}
}

func errPipeColNotFound(col string, available []string, src string) *ShellError {
	return &ShellError{
		Code:    "E013",
		Kind:    "ColumnNotFound",
		Message: fmt.Sprintf("column %q does not exist", col),
		Source:  src,
		Col:     strings.Index(src, col),
		Hint:    fmt.Sprintf("Available columns: %s", strings.Join(available, ", ")),
		Fix:     "ls | select name,size  (use existing column names)",
	}
}

func errPipeUsage(op, usage, src string) *ShellError {
	return &ShellError{
		Code:    "E014",
		Kind:    "PipeUsageError",
		Message: fmt.Sprintf("%s: %s", op, usage),
		Source:  src,
		Col:     strings.Index(src, op),
		Hint:    fmt.Sprintf("Usage: %s", usage),
		Fix:     getExampleForPipe(op),
	}
}

func errPipeInvalidNum(op, val, src string) *ShellError {
	return &ShellError{
		Code:    "E015",
		Kind:    "InvalidNumber",
		Message: fmt.Sprintf("%s: invalid number %q", op, val),
		Source:  src,
		Col:     strings.Index(src, val),
		Hint:    "Expected a non-negative integer",
		Fix:     "limit 10  (use a positive number)",
	}
}

func errPipeFormat(format, src string) *ShellError {
	return &ShellError{
		Code:    "E016",
		Kind:    "InvalidFormat",
		Message: fmt.Sprintf("unknown format %q (supported: json, csv, tsv)", format),
		Source:  src,
		Col:     strings.Index(src, format),
		Hint:    "Available formats: json, csv, tsv",
		Fix:     "ls | fmt json  |  ls | fmt csv  |  ls | fmt tsv",
	}
}

func getExampleForPipe(op string) string {
	examples := map[string]string{
		"select":    "ls | select name,size",
		"cols":      "ls | cols name,size",
		"where":     "ls | where size>1000",
		"filter":    "ls | filter name=foo",
		"grep":      "ls | grep .txt",
		"search":    "ls | search pattern",
		"sort":      "ls | sort size desc",
		"orderby":   "ls | orderby name",
		"limit":     "ls | limit 10",
		"head":      "ls | head 5",
		"top":       "ls | top 20",
		"skip":      "ls | skip 5",
		"offset":    "ls | offset 10",
		"tail":      "ls | tail 5",
		"unique":    "ls | unique type",
		"distinct":  "ls | distinct",
		"reverse":   "ls | reverse",
		"fmt":       "ls | fmt json",
		"format":    "ls | format csv",
		"add":       "ls | add newcol=value",
		"addcol":    "ls | addcol status=active",
		"rename":    "ls | rename old=new",
		"renamecol": "ls | renamecol a=b",
	}
	if ex, ok := examples[op]; ok {
		return ex
	}
	return "see help for pipe syntax"
}

// ── Execution error constructors ─────────────────────────────────────────────

func errExecFailed(cmd, msg, src string) *ShellError {
	return &ShellError{
		Code:    "E017",
		Kind:    "ExecutionFailed",
		Message: fmt.Sprintf("command failed: %s", cmd),
		Detail:  msg,
		Source:  src,
		Col:     0,
		Hint:    "Check the command syntax and arguments",
		Fix:     "Try running the command directly in terminal to debug",
	}
}

func errNoInput(cmd, src string) *ShellError {
	return &ShellError{
		Code:    "E018",
		Kind:    "NoInput",
		Message: fmt.Sprintf("%s requires table input but got text", cmd),
		Source:  src,
		Col:     0,
		Hint:    "Some pipes require a table (ls, ps, find -ls, etc.)",
		Fix:     "ls | select name  |  where size>0  (must start with table-producing command)",
	}
}

// ── Parse error constructors ────────────────────────────────────────────────

func errParseQuote(src string) *ShellError {
	return &ShellError{
		Code:    "E019",
		Kind:    "UnclosedQuote",
		Message: "unclosed quote in command",
		Source:  src,
		Col:     findQuotePos(src),
		Hint:    "Make sure all quotes (single or double) are properly closed",
		Fix:     "echo \"hello world\"  (matching pair of quotes)",
	}
}

func errParsePipe(src string) *ShellError {
	return &ShellError{
		Code:    "E020",
		Kind:    "InvalidPipe",
		Message: "pipe operator | must separate two commands",
		Source:  src,
		Col:     strings.Index(src, "|"),
		Hint:    "Check for missing command before or after the pipe",
		Fix:     "ls | grep .txt  (command | pipe | command)",
	}
}

func findQuotePos(s string) int {
	inQuote := false
	quoteChar := rune(0)
	for i, ch := range s {
		switch {
		case inQuote:
			if ch == quoteChar {
				return -1
			}
		case ch == '"' || ch == '\'':
			inQuote = true
			quoteChar = ch
			return i
		}
	}
	return -1
}

// ── Scripting error constructors ────────────────────────────────────────────

func errScriptSyntax(msg, src string) *ShellError {
	return &ShellError{
		Code:    "E021",
		Kind:    "ScriptSyntaxError",
		Message: msg,
		Source:  src,
		Col:     0,
		Hint:    "Check script syntax (colons, indentation, keywords)",
		Fix:     "if x > 0: echo yes  |  for i in range(10): echo i  |  func add(a,b) { return a + b }",
	}
}

func errScriptFunc(name, msg, src string) *ShellError {
	return &ShellError{
		Code:    "E022",
		Kind:    "FunctionError",
		Message: fmt.Sprintf("function %s: %s", name, msg),
		Source:  src,
		Col:     strings.Index(src, name),
		Hint:    "Check function definition and arguments",
	}
}
