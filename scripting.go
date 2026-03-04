package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// ─────────────────────────────────────────────────────────────────────────────
//  StructSH Scripting Engine
//
//  Supported syntax:
//
//  Variables
//    x = 42
//    x = "hello"
//    x = `ls -la | count`      backtick = subshell capture
//    x = if cond: val; else: val2
//    z++  /  z--  /  ++z  /  --z
//    z += 1  /  z -= 1  /  z *= 2  /  z /= 2  /  z %= 3
//
//  If / elif / else
//    if x >= 10: echo "big"
//    if x > 0: echo $x; elif x == 0: echo "zero"; else: echo "neg"
//    if USER != "": { echo hello }; else { echo nobody }
//
//  For
//    for i in range(0, 10): echo $i
//    for i in range(1..100): echo $i
//    for name in `ls | select name`: echo $name
//
//  While
//    while x < 100: x += 1
//    while alive != false: { echo running; sleep 1 }
//
//  String ops
//    echo "hello"x3                  → hellohellohello
//    echo "A"x100 . "B" . `whoami`   → AAAA...BBBHOSTNAME
//
//  Func definition
//    func greet(name) { echo "Hello, $name" }
//    func add(a, b) { return $a + $b }
//
//  Func call
//    greet "world"
//    result = add 3 7
//
//  Backtick subshell in any position
//    echo `whoami`
//    files = `ls -la | where type=dir | select name`
// ─────────────────────────────────────────────────────────────────────────────

// UserFunc is a user-defined function.
type UserFunc struct {
	Name   string
	Params []string
	Body   []string // lines of body
}

// evalLine is the scripting entry point called from execLine.
// Returns (handled, exitCode).
func (sh *Shell) evalScript(raw string) (bool, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "///") {
		return true, 0
	}

	// ── Compound: if / for / while / func ───────────────────────────────────
	lower := strings.ToLower(raw)

	if strings.HasPrefix(lower, "if ") || strings.HasPrefix(lower, "if(") {
		return true, sh.evalIf(raw)
	}
	if strings.HasPrefix(lower, "for ") {
		return true, sh.evalFor(raw)
	}
	if strings.HasPrefix(lower, "while ") {
		return true, sh.evalWhile(raw)
	}
	if strings.HasPrefix(lower, "func ") {
		return true, sh.evalFuncDef(raw)
	}

	// ── print keyword ────────────────────────────────────────────────────────
	if strings.HasPrefix(lower, "print ") {
		text := expandStringExpr(sh, raw[6:])
		fmt.Println("  " + text)
		return true, 0
	}

	// ── Variable assignment ───────────────────────────────────────────────────
	if code, ok := sh.tryVarAssign(raw); ok {
		return true, code
	}

	// ── In-place operators:  z++  z--  ++z  --z ─────────────────────────────
	if code, ok := sh.tryIncrDecr(raw); ok {
		return true, code
	}

	// ── User function call ────────────────────────────────────────────────────
	parts := strings.Fields(raw)
	if len(parts) > 0 {
		if fn, ok := sh.funcs[parts[0]]; ok {
			return true, sh.callUserFunc(fn, parts[1:], raw)
		}
	}

	return false, 0
}

// ─────────────────────────────────────────────────────────────────────────────
//  Variable assignment
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) tryVarAssign(raw string) (int, bool) {
	// Compound operators:  x += y  x -= y  x *= y  x /= y  x %= y
	for _, op := range []string{"+=", "-=", "*=", "/=", "%="} {
		if idx := strings.Index(raw, op); idx > 0 {
			name := strings.TrimSpace(raw[:idx])
			if isIdent(name) {
				rhs := strings.TrimSpace(raw[idx+2:])
				cur := sh.getVar(name)
				curF, _ := strconv.ParseFloat(cur, 64)
				rhsF, err := strconv.ParseFloat(sh.evalExpr(rhs), 64)
				if err != nil {
					PrintError(errType(fmt.Sprintf("cannot apply %s to non-numeric value %q", op, rhs), raw))
					return 1, true
				}
				var result float64
				switch op {
				case "+=": result = curF + rhsF
				case "-=": result = curF - rhsF
				case "*=": result = curF * rhsF
				case "/=":
					if rhsF == 0 { PrintError(errDivZero(raw)); return 1, true }
					result = curF / rhsF
				case "%=":
					if rhsF == 0 { PrintError(errDivZero(raw)); return 1, true }
					result = math.Mod(curF, rhsF)
				}
				sh.setVar(name, fmtNum(result))
				return 0, true
			}
		}
	}

	// Simple assignment:  name = value
	idx := strings.Index(raw, "=")
	if idx <= 0 { return 0, false }
	// Make sure it's not == != <= >=
	if idx > 0 && (raw[idx-1] == '!' || raw[idx-1] == '<' || raw[idx-1] == '>') { return 0, false }
	if idx+1 < len(raw) && raw[idx+1] == '=' { return 0, false }

	name := strings.TrimSpace(raw[:idx])
	if !isIdent(name) { return 0, false }

	rhs := strings.TrimSpace(raw[idx+1:])
	val := sh.evalRHS(rhs, raw)
	sh.setVar(name, val)
	return 0, true
}

func (sh *Shell) tryIncrDecr(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	// postfix: z++  z--
	if strings.HasSuffix(raw, "++") {
		name := strings.TrimSpace(raw[:len(raw)-2])
		if isIdent(name) { sh.incrVar(name, 1); return 0, true }
	}
	if strings.HasSuffix(raw, "--") {
		name := strings.TrimSpace(raw[:len(raw)-2])
		if isIdent(name) { sh.incrVar(name, -1); return 0, true }
	}
	// prefix: ++z  --z
	if strings.HasPrefix(raw, "++") {
		name := strings.TrimSpace(raw[2:])
		if isIdent(name) { sh.incrVar(name, 1); return 0, true }
	}
	if strings.HasPrefix(raw, "--") {
		name := strings.TrimSpace(raw[2:])
		if isIdent(name) { sh.incrVar(name, -1); return 0, true }
	}
	return 0, false
}

func (sh *Shell) incrVar(name string, delta float64) {
	cur, _ := strconv.ParseFloat(sh.getVar(name), 64)
	sh.setVar(name, fmtNum(cur+delta))
}

// evalRHS evaluates the right-hand side of an assignment.
// Handles: string literals, inline if, backtick subshell, arithmetic, plain values.
func (sh *Shell) evalRHS(rhs, src string) string {
	rhs = strings.TrimSpace(rhs)

	// Inline if:  x = if cond: val; else: val2
	if strings.HasPrefix(strings.ToLower(rhs), "if ") {
		return sh.evalInlineIf(rhs, src)
	}

	// Backtick subshell: `cmd`
	if strings.HasPrefix(rhs, "`") && strings.HasSuffix(rhs, "`") {
		return sh.runSubshell(rhs[1:len(rhs)-1])
	}

	// String literal
	if (strings.HasPrefix(rhs, `"`) && strings.HasSuffix(rhs, `"`)) ||
		(strings.HasPrefix(rhs, `'`) && strings.HasSuffix(rhs, `'`)) {
		return rhs[1 : len(rhs)-1]
	}

	// Arithmetic / expression
	return sh.evalExpr(rhs)
}

// ─────────────────────────────────────────────────────────────────────────────
//  Expression evaluator (numeric + string + variable)
// ─────────────────────────────────────────────────────────────────────────────

// evalExpr resolves a value: variable name, number, arithmetic, string.
func (sh *Shell) evalExpr(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" { return "" }

	// Variable expansion $VAR or bare VAR that exists
	if strings.HasPrefix(expr, "$") {
		return sh.getVar(expr[1:])
	}
	if val, ok := sh.vars[expr]; ok {
		return val
	}

	// String with ops: "A"x3  "A" . "B" . `cmd`
	if strings.ContainsAny(expr, `"'`) || strings.Contains(expr, "` ") {
		return expandStringExpr(sh, expr)
	}

	// Arithmetic: detect +  -  *  /  %  ^  ops between tokens
	if result, ok := tryArith(sh, expr); ok {
		return result
	}

	// Backtick
	if strings.HasPrefix(expr, "`") && strings.HasSuffix(expr, "`") {
		return sh.runSubshell(expr[1:len(expr)-1])
	}

	return expr
}

// tryArith tries simple arithmetic on an expression like "3 + 4" or "$x * 2".
func tryArith(sh *Shell, expr string) (string, bool) {
	// Operators in reverse precedence order for simple left-to-right
	for _, op := range []string{"+", "-", "*", "/", "%"} {
		// Find last occurrence to handle left-to-right
		idx := strings.LastIndex(expr, " "+op+" ")
		if idx < 0 { continue }
		lhs := strings.TrimSpace(expr[:idx])
		rhs := strings.TrimSpace(expr[idx+3:])
		lv, _ := strconv.ParseFloat(sh.evalExpr(lhs), 64)
		rv, _ := strconv.ParseFloat(sh.evalExpr(rhs), 64)
		var result float64
		switch op {
		case "+": result = lv + rv
		case "-": result = lv - rv
		case "*": result = lv * rv
		case "/":
			if rv == 0 { return "0", true }
			result = lv / rv
		case "%":
			if rv == 0 { return "0", true }
			result = math.Mod(lv, rv)
		}
		return fmtNum(result), true
	}
	return "", false
}

// expandStringExpr handles string concatenation: "A"x3 . "B" . `cmd`
func expandStringExpr(sh *Shell, expr string) string {
	// Split on  .  outside quotes/backticks
	parts := splitOnDot(expr)
	var sb strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		sb.WriteString(evalStringPart(sh, p))
	}
	return sb.String()
}

func evalStringPart(sh *Shell, p string) string {
	// "text"xN repetition
	if xIdx := strings.LastIndex(p, `"x`); xIdx > 0 {
		strPart := p[:xIdx+1]
		numPart := p[xIdx+2:]
		n := 0
		fmt.Sscanf(numPart, "%d", &n)
		text := stripQuotes(strPart)
		return strings.Repeat(text, n)
	}
	// backtick
	if strings.HasPrefix(p, "`") && strings.HasSuffix(p, "`") {
		return strings.TrimSpace(sh.runSubshell(p[1:len(p)-1]))
	}
	// $VAR
	if strings.HasPrefix(p, "$") {
		return sh.getVar(p[1:])
	}
	return stripQuotes(sh.expandVars(p))
}

func splitOnDot(s string) []string {
	var parts []string
	var cur strings.Builder
	inQ := false
	qCh := rune(0)
	inBt := false
	for _, ch := range s {
		switch {
		case inBt:
			cur.WriteRune(ch)
			if ch == '`' { inBt = false }
		case inQ:
			cur.WriteRune(ch)
			if ch == qCh { inQ = false }
		case ch == '`':
			inBt = true; cur.WriteRune(ch)
		case ch == '"' || ch == '\'':
			inQ = true; qCh = ch; cur.WriteRune(ch)
		case ch == '.':
			parts = append(parts, cur.String()); cur.Reset()
		default:
			cur.WriteRune(ch)
		}
	}
	if cur.Len() > 0 { parts = append(parts, cur.String()) }
	return parts
}

func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
//  Condition evaluator
// ─────────────────────────────────────────────────────────────────────────────

// evalCond evaluates a boolean condition: "x >= 10", "USER != \"\"", "alive == true"
func (sh *Shell) evalCond(cond string) bool {
	cond = strings.TrimSpace(cond)

	for _, op := range []string{"!=", ">=", "<=", "==", ">", "<", "~"} {
		idx := strings.Index(cond, op)
		if idx <= 0 { continue }
		lhs := strings.TrimSpace(cond[:idx])
		rhs := strings.TrimSpace(cond[idx+len(op):])
		lv := sh.evalExpr(lhs)
		rv := stripQuotes(sh.evalExpr(rhs))
		lv = strings.TrimSpace(lv)
		rv = strings.TrimSpace(rv)

		lf, lIsNum := parseNum(lv)
		rf, rIsNum := parseNum(rv)

		switch op {
		case "==": return lv == rv
		case "!=": return lv != rv
		case "~":  return strings.Contains(lv, rv)
		case ">":
			if lIsNum && rIsNum { return lf > rf }
			return lv > rv
		case "<":
			if lIsNum && rIsNum { return lf < rf }
			return lv < rv
		case ">=":
			if lIsNum && rIsNum { return lf >= rf }
			return lv >= rv
		case "<=":
			if lIsNum && rIsNum { return lf <= rf }
			return lv <= rv
		}
	}

	// Bare true/false/non-empty
	v := sh.evalExpr(cond)
	switch strings.ToLower(v) {
	case "true", "1", "yes": return true
	case "false", "0", "no", "": return false
	}
	return v != ""
}

// ─────────────────────────────────────────────────────────────────────────────
//  If / elif / else
//  Syntax (all on one line):
//    if cond: body
//    if cond: body; elif cond2: body2; else: body3
//    if cond: { multi; word; body }; else { body }
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) evalIf(raw string) int {
	// Normalise: remove leading "if "
	rest := strings.TrimSpace(raw[3:])

	// Extract (cond, body, rest) triplets
	type branch struct{ cond, body string }
	var branches []branch
	var elsebody string

	// tokenise on semicolons outside braces/quotes
	clauses := splitSemicolon(rest)

	i := 0
	for i < len(clauses) {
		cl := strings.TrimSpace(clauses[i])
		low := strings.ToLower(cl)
		if strings.HasPrefix(low, "elif ") || strings.HasPrefix(low, "else if ") {
			// elif cond: body
			offset := 5
			if strings.HasPrefix(low, "else if ") { offset = 8 }
			inner := cl[offset:]
			cond, body := splitColon(inner)
			branches = append(branches, branch{cond: cond, body: extractBody(body)})
		} else if strings.HasPrefix(low, "else:") || strings.HasPrefix(low, "else ") || low == "else" || strings.HasPrefix(low, "else{") {
			// else: body or else { body }
			after := strings.TrimSpace(cl[4:])
			after = strings.TrimPrefix(after, ":")
			elsebody = extractBody(strings.TrimSpace(after))
		} else {
			// if cond: body  (first clause)
			cond, body := splitColon(cl)
			branches = append(branches, branch{cond: cond, body: extractBody(body)})
		}
		i++
	}

	// Evaluate
	for _, br := range branches {
		if sh.evalCond(br.cond) {
			return sh.execBodyLines(br.body)
		}
	}
	if elsebody != "" {
		return sh.execBodyLines(elsebody)
	}
	return 0
}

// evalInlineIf evaluates:  if cond: val; else: val2
// Returns the string value.
func (sh *Shell) evalInlineIf(raw, src string) string {
	rest := strings.TrimSpace(raw[3:])
	parts := splitSemicolon(rest)
	var cond, thenVal, elseVal string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		low := strings.ToLower(p)
		if strings.HasPrefix(low, "else:") || strings.HasPrefix(low, "else ") {
			elseVal = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p[4:]), ":"))
		} else {
			c, body := splitColon(p)
			if cond == "" { cond = c; thenVal = body }
		}
	}
	if sh.evalCond(cond) {
		return sh.evalRHS(thenVal, src)
	}
	return sh.evalRHS(elseVal, src)
}

// ─────────────────────────────────────────────────────────────────────────────
//  For loop
//  Syntax: for x in range(0, 10): body
//          for x in range(0..99): body
//          for x in `cmd`: body    (iterates over lines)
//          for x in a b c: body
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) evalFor(raw string) int {
	rest := strings.TrimSpace(raw[4:]) // strip "for "

	// Parse:  varname in iterable: body
	inIdx := strings.Index(strings.ToLower(rest), " in ")
	if inIdx < 0 {
		PrintError(errSyntax("expected 'for <var> in <iterable>: <body>'", raw, 0))
		return 1
	}
	varName := strings.TrimSpace(rest[:inIdx])
	after   := strings.TrimSpace(rest[inIdx+4:])
	colonIdx := colonOutsideBraces(after)
	if colonIdx < 0 {
		PrintError(errSyntax("expected ':' after iterable in for loop", raw, inIdx+4))
		return 1
	}
	iterExpr := strings.TrimSpace(after[:colonIdx])
	body     := extractBody(strings.TrimSpace(after[colonIdx+1:]))

	items := sh.evalIterable(iterExpr, raw)

	for _, item := range items {
		sh.setVar(varName, item)
		code := sh.execBodyLines(body)
		if code == codeBreak    { break }
		if code == codeContinue { continue }
		if code != 0 && code != codeContinue { return code }
	}
	sh.delVar(varName)
	return 0
}

// evalIterable expands an iterable expression to a string slice.
func (sh *Shell) evalIterable(expr, src string) []string {
	expr = strings.TrimSpace(expr)

	// range(a, b)  or  range(a..b)
	if strings.HasPrefix(strings.ToLower(expr), "range") {
		inner := expr[5:]
		inner = strings.Trim(inner, "()")
		// x..y
		if strings.Contains(inner, "..") {
			parts := strings.SplitN(inner, "..", 2)
			a, b := 0, 0
			fmt.Sscanf(strings.TrimSpace(sh.evalExpr(parts[0])), "%d", &a)
			fmt.Sscanf(strings.TrimSpace(sh.evalExpr(parts[1])), "%d", &b)
			return makeRange(a, b)
		}
		// a, b
		parts := strings.SplitN(inner, ",", 2)
		a, b := 0, 0
		fmt.Sscanf(strings.TrimSpace(sh.evalExpr(parts[0])), "%d", &a)
		if len(parts) > 1 {
			fmt.Sscanf(strings.TrimSpace(sh.evalExpr(parts[1])), "%d", &b)
		}
		return makeRange(a, b)
	}

	// backtick subshell — iterate over lines
	if strings.HasPrefix(expr, "`") && strings.HasSuffix(expr, "`") {
		out := sh.runSubshell(expr[1:len(expr)-1])
		var lines []string
		for _, l := range strings.Split(out, "\n") {
			if t := strings.TrimSpace(l); t != "" {
				lines = append(lines, t)
			}
		}
		return lines
	}

	// $var that holds a space-separated list
	if strings.HasPrefix(expr, "$") {
		val := sh.getVar(expr[1:])
		return strings.Fields(val)
	}

	// bare space-separated values
	return strings.Fields(expr)
}

func makeRange(a, b int) []string {
	if a > b {
		out := make([]string, a-b)
		for i := range out { out[i] = strconv.Itoa(a-i) }
		return out
	}
	out := make([]string, b-a)
	for i := range out { out[i] = strconv.Itoa(a+i) }
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
//  While loop
//  Syntax: while cond: body
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) evalWhile(raw string) int {
	rest := strings.TrimSpace(raw[6:]) // strip "while "
	colonIdx := colonOutsideBraces(rest)
	if colonIdx < 0 {
		PrintError(errSyntax("expected ':' in while statement", raw, 0))
		return 1
	}
	cond := strings.TrimSpace(rest[:colonIdx])
	body := extractBody(strings.TrimSpace(rest[colonIdx+1:]))

	maxIter := 100000
	for i := 0; i < maxIter; i++ {
		if !sh.evalCond(cond) { break }
		code := sh.execBodyLines(body)
		if code == codeBreak    { break }
		if code == codeContinue { continue }
		if code != 0            { return code }
	}
	return 0
}

// ─────────────────────────────────────────────────────────────────────────────
//  Func definition
//  Syntax: func name(param1, param2) { body }
//          func name(arr[]) { body }
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) evalFuncDef(raw string) int {
	rest := strings.TrimSpace(raw[5:]) // strip "func "

	// Find name
	parenIdx := strings.IndexAny(rest, "( {")
	if parenIdx < 0 {
		PrintError(errSyntax("expected '(' after func name", raw, 5))
		return 1
	}
	name := strings.TrimSpace(rest[:parenIdx])

	// Parse params: (a, b, c)  or  ()
	var params []string
	if rest[parenIdx] == '(' {
		closeIdx := strings.Index(rest, ")")
		if closeIdx < 0 {
			PrintError(errSyntax("unclosed '(' in func definition", raw, parenIdx))
			return 1
		}
		paramStr := rest[parenIdx+1:closeIdx]
		for _, p := range strings.Split(paramStr, ",") {
			p = strings.TrimSpace(p)
			p = strings.TrimSuffix(p, "[]") // strip array marker
			if p != "" { params = append(params, p) }
		}
		rest = strings.TrimSpace(rest[closeIdx+1:])
	}

	// Extract body: { ... }
	body := extractBody(rest)

	sh.funcs[name] = &UserFunc{Name: name, Params: params, Body: bodyLines(body)}
	fmt.Printf("  %s✔ defined func %s%s%s(%s)%s\n",
		ansiGreen, ansiBold+ansiCyan, name, ansiReset,
		strings.Join(params, ", "), ansiReset)
	return 0
}

func (sh *Shell) callUserFunc(fn *UserFunc, args []string, src string) int {
	// Save + set params
	saved := make(map[string]string)
	for i, p := range fn.Params {
		saved[p] = sh.vars[p]
		if i < len(args) {
			sh.vars[p] = sh.evalExpr(args[i])
		} else {
			sh.vars[p] = ""
		}
	}
	// Execute body
	code := 0
	for _, line := range fn.Body {
		line = strings.TrimSpace(line)
		if line == "" { continue }
		code = sh.execLine(line)
		if code == codeReturn { code = 0; break }
	}
	// Restore
	for p, v := range saved {
		sh.vars[p] = v
	}
	return code
}

// ─────────────────────────────────────────────────────────────────────────────
//  Backtick subshell expansion
//  echo `ls -la | where type=dir | select name`
// ─────────────────────────────────────────────────────────────────────────────

// runSubshell executes a command and returns its text output.
func (sh *Shell) runSubshell(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" { return "" }

	// Capture output by redirecting printResult
	old := sh.captureMode
	sh.captureMode = true
	sh.captureOut.Reset()

	sh.execLine(cmd)

	sh.captureMode = false
	out := strings.TrimRight(sh.captureOut.String(), "\n ")
	sh.captureOut.Reset()
	sh.captureMode = old
	return out
}

// expandBackticks replaces all `...` in a string with their output.
func (sh *Shell) expandBackticks(s string) string {
	for {
		start := strings.Index(s, "`")
		if start < 0 { break }
		end := strings.Index(s[start+1:], "`")
		if end < 0 { break }
		end += start + 1
		cmd := s[start+1:end]
		result := sh.runSubshell(cmd)
		s = s[:start] + result + s[end+1:]
	}
	return s
}

// ─────────────────────────────────────────────────────────────────────────────
//  Body execution helpers
// ─────────────────────────────────────────────────────────────────────────────

const (
	codeBreak    = -1
	codeContinue = -2
	codeReturn   = -3
)

// execBodyLines runs a multi-line body string.
func (sh *Shell) execBodyLines(body string) int {
	for _, line := range bodyLines(body) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") { continue }
		if strings.ToLower(line) == "break"    { return codeBreak }
		if strings.ToLower(line) == "continue" { return codeContinue }
		if strings.HasPrefix(strings.ToLower(line), "return") {
			val := strings.TrimSpace(line[6:])
			if val != "" { sh.setVar("_return", sh.evalExpr(val)) }
			return codeReturn
		}
		code := sh.execLine(line)
		if code != 0 { return code }
	}
	return 0
}

func bodyLines(body string) []string {
	// Remove surrounding braces if present
	body = strings.TrimSpace(body)
	if strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}") {
		body = body[1:len(body)-1]
	}
	// Split on ; and \n
	var lines []string
	for _, line := range strings.Split(body, ";") {
		for _, sub := range strings.Split(line, "\n") {
			if t := strings.TrimSpace(sub); t != "" {
				lines = append(lines, t)
			}
		}
	}
	return lines
}

// extractBody unwraps { body } or returns the raw string.
func extractBody(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		depth := 0
		for i, ch := range s {
			if ch == '{' { depth++ }
			if ch == '}' { depth--; if depth == 0 { return s[:i+1] } }
		}
	}
	return s
}

// splitColon splits "cond: body" at first colon outside brackets.
func splitColon(s string) (cond, body string) {
	idx := colonOutsideBraces(s)
	if idx < 0 { return s, "" }
	return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
}

// colonOutsideBraces finds the first ':' not inside {}, (), []
func colonOutsideBraces(s string) int {
	depth := 0
	inQ   := false
	qCh   := rune(0)
	for i, ch := range s {
		if inQ { if ch == qCh { inQ = false }; continue }
		if ch == '"' || ch == '\'' { inQ = true; qCh = ch; continue }
		if ch == '{' || ch == '(' || ch == '[' { depth++; continue }
		if ch == '}' || ch == ')' || ch == ']' { depth--; continue }
		if ch == ':' && depth == 0 { return i }
	}
	return -1
}

// splitSemicolon splits on ';' outside braces/quotes.
func splitSemicolon(s string) []string {
	var parts []string
	var cur strings.Builder
	depth := 0
	inQ   := false
	qCh   := rune(0)
	for _, ch := range s {
		if inQ { cur.WriteRune(ch); if ch == qCh { inQ = false }; continue }
		if ch == '"' || ch == '\'' { inQ = true; qCh = ch; cur.WriteRune(ch); continue }
		if ch == '{' || ch == '(' { depth++; cur.WriteRune(ch); continue }
		if ch == '}' || ch == ')' { depth--; cur.WriteRune(ch); continue }
		if ch == ';' && depth == 0 {
			if t := strings.TrimSpace(cur.String()); t != "" { parts = append(parts, t) }
			cur.Reset()
		} else { cur.WriteRune(ch) }
	}
	if t := strings.TrimSpace(cur.String()); t != "" { parts = append(parts, t) }
	return parts
}

// ─────────────────────────────────────────────────────────────────────────────
//  Variable helpers
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) getVar(name string) string {
	if v, ok := sh.vars[name]; ok { return v }
	return os.Getenv(name)
}

func (sh *Shell) setVar(name, val string) {
	sh.vars[name] = val
}

func (sh *Shell) delVar(name string) {
	delete(sh.vars, name)
}

// ─────────────────────────────────────────────────────────────────────────────
//  Misc helpers
// ─────────────────────────────────────────────────────────────────────────────

func isIdent(s string) bool {
	if s == "" { return false }
	for i, ch := range s {
		if i == 0 && !unicode.IsLetter(ch) && ch != '_' { return false }
		if i > 0 && !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '_' { return false }
	}
	return true
}

func parseNum(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func fmtNum(f float64) string {
	if f == math.Trunc(f) { return strconv.FormatInt(int64(f), 10) }
	return strconv.FormatFloat(f, 'f', -1, 64)
}
