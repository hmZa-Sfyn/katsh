package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ─────────────────────────────────────────────
//  Pipe transforms applied after command execution.
//  Each function takes a *Result and returns a *Result.
// ─────────────────────────────────────────────

// ApplyPipes runs all pipe stages in sequence on a Result.
func ApplyPipes(r *Result, pipes []PipeStage) (*Result, error) {
	var err error
	for _, p := range pipes {
		r, err = applyPipe(r, p)
		if err != nil {
			return r, fmt.Errorf("pipe '%s': %w", p.Op, err)
		}
	}
	return r, nil
}

func applyPipe(r *Result, p PipeStage) (*Result, error) {
	switch p.Op {
	case "select", "cols":
		return pipeSelect(r, p.Args)
	case "where", "filter":
		return pipeWhere(r, p.Args)
	case "grep", "search":
		return pipeGrep(r, p.Args)
	case "sort", "orderby", "order":
		return pipeSort(r, p.Args)
	case "limit", "head", "top":
		return pipeLimit(r, p.Args)
	case "skip", "offset", "tail":
		return pipeSkip(r, p.Args)
	case "count":
		return pipeCount(r)
	case "unique", "distinct":
		return pipeUnique(r, p.Args)
	case "reverse":
		return pipeReverse(r)
	case "fmt", "format":
		return pipeFmt(r, p.Args)
	case "add", "addcol":
		return pipeAddCol(r, p.Args)
	case "rename", "renamecol":
		return pipeRenameCol(r, p.Args)
	default:
		return r, errUnknownPipe(p.Op, p.Op)
	}
}

// ── select ────────────────────────────────────
// | select col1,col2,col3

func pipeSelect(r *Result, cols []string) (*Result, error) {
	if !r.IsTable {
		return r, nil
	}
	existing := make(map[string]bool, len(r.Cols))
	for _, c := range r.Cols {
		existing[c] = true
	}
	var valid []string
	for _, c := range cols {
		if existing[c] {
			valid = append(valid, c)
		} else {
			return r, errPipeColNotFound(c, r.Cols, fmt.Sprintf("select %s", strings.Join(cols, ",")))
		}
	}
	if len(valid) == 0 {
		return NewText("select: no matching columns"), nil
	}
	rows := make([]Row, len(r.Rows))
	for i, row := range r.Rows {
		nr := make(Row, len(valid))
		for _, c := range valid {
			nr[c] = row[c]
		}
		rows[i] = nr
	}
	return NewTable(valid, rows), nil
}

// ── where ─────────────────────────────────────
// | where col=val
// | where col!=val
// | where col>val  (numeric)
// | where col<val  (numeric)
// | where col>=val
// | where col<=val
// | where col~pattern  (contains, case-insensitive)

func pipeWhere(r *Result, args []string) (*Result, error) {
	if !r.IsTable {
		return r, errNoInput("where", "where "+strings.Join(args, " "))
	}
	if len(args) == 0 {
		return r, errPipeUsage("where", "expected condition (e.g., col=value, col>10, col~pattern)", "where")
	}
	expr := strings.Join(args, " ")

	col, op, needle, err := parseWhereExpr(expr)
	if err != nil {
		return r, err
	}
	col = strings.ToLower(col)

	// Validate column exists
	colExists := false
	for _, c := range r.Cols {
		if c == col {
			colExists = true
			break
		}
	}
	if !colExists {
		return r, errPipeColNotFound(col, r.Cols, expr)
	}

	var rows []Row
	for _, row := range r.Rows {
		cell := row[col]
		if matchesWhere(cell, op, needle) {
			rows = append(rows, row)
		}
	}
	return NewTable(r.Cols, rows), nil
}

// parseWhereExpr parses "col>=value" into (col, op, value).
func parseWhereExpr(expr string) (col, op, val string, err error) {
	ops := []string{"!=", ">=", "<=", "~", ">", "<", "="}
	for _, candidate := range ops {
		idx := strings.Index(expr, candidate)
		if idx > 0 {
			return strings.TrimSpace(expr[:idx]),
				candidate,
				strings.TrimSpace(expr[idx+len(candidate):]),
				nil
		}
	}
	return "", "", "", errPipeUsage("where", "use col=value, col!=value, col>value, col~pattern", expr)
}

func matchesWhere(cell, op, needle string) bool {
	cellLow := strings.ToLower(strings.TrimSpace(cell))
	needleLow := strings.ToLower(strings.TrimSpace(needle))

	switch op {
	case "=":
		return cellLow == needleLow
	case "!=":
		return cellLow != needleLow
	case "~":
		return strings.Contains(cellLow, needleLow)
	case ">", "<", ">=", "<=":
		cf := parseFloat(strings.TrimSuffix(cellLow, "%"))
		nf := parseFloat(strings.TrimSuffix(needleLow, "%"))
		switch op {
		case ">":
			return cf > nf
		case "<":
			return cf < nf
		case ">=":
			return cf >= nf
		case "<=":
			return cf <= nf
		}
	}
	return false
}

// ── grep ──────────────────────────────────────
// | grep text   (searches all columns / all lines)

func pipeGrep(r *Result, args []string) (*Result, error) {
	if len(args) == 0 {
		return r, errPipeUsage("grep", "expected search pattern", "grep")
	}
	needle := strings.ToLower(strings.Join(args, " "))

	if !r.IsTable {
		var lines []string
		for _, line := range strings.Split(r.Text, "\n") {
			if strings.Contains(strings.ToLower(line), needle) {
				lines = append(lines, line)
			}
		}
		return NewText(strings.Join(lines, "\n")), nil
	}

	var rows []Row
	for _, row := range r.Rows {
		for _, v := range row {
			if strings.Contains(strings.ToLower(v), needle) {
				rows = append(rows, row)
				break
			}
		}
	}
	return NewTable(r.Cols, rows), nil
}

// ── sort ──────────────────────────────────────
// | sort col [asc|desc]

func pipeSort(r *Result, args []string) (*Result, error) {
	if !r.IsTable {
		return r, nil
	}
	if len(args) == 0 {
		return r, errPipeUsage("sort", "expected column name [asc|desc]", "sort")
	}
	col := strings.ToLower(args[0])
	desc := len(args) > 1 && strings.ToLower(args[1]) == "desc"

	// Copy rows
	rows := make([]Row, len(r.Rows))
	copy(rows, r.Rows)

	sort.SliceStable(rows, func(i, j int) bool {
		a := rows[i][col]
		b := rows[j][col]
		// Try numeric sort
		af := parseFloat(strings.TrimSuffix(strings.TrimSuffix(a, "%"), "B"))
		bf := parseFloat(strings.TrimSuffix(strings.TrimSuffix(b, "%"), "B"))
		var less bool
		if af != 0 || bf != 0 {
			less = af < bf
		} else {
			less = strings.ToLower(a) < strings.ToLower(b)
		}
		if desc {
			return !less
		}
		return less
	})

	return NewTable(r.Cols, rows), nil
}

// ── limit ─────────────────────────────────────
// | limit N

func pipeLimit(r *Result, args []string) (*Result, error) {
	if len(args) == 0 {
		return r, errPipeUsage("limit", "expected number N", "limit")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 0 {
		return r, errPipeInvalidNum("limit", args[0], "limit "+args[0])
	}
	if !r.IsTable {
		lines := strings.Split(r.Text, "\n")
		if n > len(lines) {
			n = len(lines)
		}
		return NewText(strings.Join(lines[:n], "\n")), nil
	}
	rows := r.Rows
	if n < len(rows) {
		rows = rows[:n]
	}
	return NewTable(r.Cols, rows), nil
}

// ── skip ──────────────────────────────────────
// | skip N

func pipeSkip(r *Result, args []string) (*Result, error) {
	if len(args) == 0 {
		return r, errPipeUsage("skip", "expected number N", "skip")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 0 {
		return r, errPipeInvalidNum("skip", args[0], "skip "+args[0])
	}
	if !r.IsTable {
		lines := strings.Split(r.Text, "\n")
		if n >= len(lines) {
			return NewText(""), nil
		}
		return NewText(strings.Join(lines[n:], "\n")), nil
	}
	rows := r.Rows
	if n >= len(rows) {
		return NewTable(r.Cols, []Row{}), nil
	}
	return NewTable(r.Cols, rows[n:]), nil
}

// ── count ─────────────────────────────────────
// | count

func pipeCount(r *Result) (*Result, error) {
	if !r.IsTable {
		n := len(strings.Split(strings.TrimSpace(r.Text), "\n"))
		if strings.TrimSpace(r.Text) == "" {
			n = 0
		}
		return NewText(fmt.Sprintf("%d", n)), nil
	}
	return NewText(fmt.Sprintf("%d", len(r.Rows))), nil
}

// ── unique ────────────────────────────────────
// | unique col   — deduplicate rows by a column value

func pipeUnique(r *Result, args []string) (*Result, error) {
	if !r.IsTable {
		return r, nil
	}
	if len(args) == 0 {
		// Deduplicate entire rows
		seen := make(map[string]bool)
		var rows []Row
		for _, row := range r.Rows {
			key := fmt.Sprintf("%v", row)
			if !seen[key] {
				seen[key] = true
				rows = append(rows, row)
			}
		}
		return NewTable(r.Cols, rows), nil
	}
	col := strings.ToLower(args[0])
	seen := make(map[string]bool)
	var rows []Row
	for _, row := range r.Rows {
		val := row[col]
		if !seen[val] {
			seen[val] = true
			rows = append(rows, row)
		}
	}
	return NewTable(r.Cols, rows), nil
}

// ── reverse ───────────────────────────────────
// | reverse

func pipeReverse(r *Result) (*Result, error) {
	if !r.IsTable {
		lines := strings.Split(r.Text, "\n")
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
		return NewText(strings.Join(lines, "\n")), nil
	}
	rows := make([]Row, len(r.Rows))
	copy(rows, r.Rows)
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return NewTable(r.Cols, rows), nil
}

// ── fmt ───────────────────────────────────────
// | fmt json
// | fmt csv
// | fmt tsv

func pipeFmt(r *Result, args []string) (*Result, error) {
	if len(args) == 0 {
		return r, errPipeUsage("fmt", "expected format (json|csv|tsv)", "fmt")
	}
	format := strings.ToLower(args[0])

	if !r.IsTable {
		return r, errNoInput("fmt", "fmt "+format)
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(r.Rows, "", "  ")
		if err != nil {
			return r, err
		}
		return NewText(string(data)), nil

	case "csv":
		var sb strings.Builder
		w := csv.NewWriter(&sb)
		_ = w.Write(r.Cols)
		for _, row := range r.Rows {
			rec := make([]string, len(r.Cols))
			for i, c := range r.Cols {
				rec[i] = row[c]
			}
			_ = w.Write(rec)
		}
		w.Flush()
		return NewText(sb.String()), nil

	case "tsv":
		var sb strings.Builder
		sb.WriteString(strings.Join(r.Cols, "\t") + "\n")
		for _, row := range r.Rows {
			vals := make([]string, len(r.Cols))
			for i, c := range r.Cols {
				vals[i] = row[c]
			}
			sb.WriteString(strings.Join(vals, "\t") + "\n")
		}
		return NewText(sb.String()), nil

	default:
		return r, errPipeFormat(format, "fmt "+format)
	}
}

// ── addcol ────────────────────────────────────
// | add colname=expression   (literal value only for now)

func pipeAddCol(r *Result, args []string) (*Result, error) {
	if !r.IsTable || len(args) == 0 {
		return r, errPipeUsage("add", "expected colname=value", "add")
	}
	parts := strings.SplitN(strings.Join(args, " "), "=", 2)
	if len(parts) != 2 {
		return r, errPipeUsage("add", "expected colname=value format", "add "+strings.Join(args, " "))
	}
	newCol := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])

	cols := append(r.Cols, newCol)
	rows := make([]Row, len(r.Rows))
	for i, row := range r.Rows {
		nr := make(Row, len(row)+1)
		for k, v := range row {
			nr[k] = v
		}
		nr[newCol] = val
		rows[i] = nr
	}
	return NewTable(cols, rows), nil
}

// ── renamecol ─────────────────────────────────
// | rename oldname=newname

func pipeRenameCol(r *Result, args []string) (*Result, error) {
	if !r.IsTable || len(args) == 0 {
		return r, errPipeUsage("rename", "expected oldcol=newcol", "rename")
	}
	parts := strings.SplitN(strings.Join(args, " "), "=", 2)
	if len(parts) != 2 {
		return r, errPipeUsage("rename", "expected oldcol=newcol format", "rename "+strings.Join(args, " "))
	}
	old := strings.TrimSpace(parts[0])
	new_ := strings.TrimSpace(parts[1])

	cols := make([]string, len(r.Cols))
	for i, c := range r.Cols {
		if c == old {
			cols[i] = new_
		} else {
			cols[i] = c
		}
	}
	rows := make([]Row, len(r.Rows))
	for i, row := range r.Rows {
		nr := make(Row, len(row))
		for k, v := range row {
			if k == old {
				nr[new_] = v
			} else {
				nr[k] = v
			}
		}
		rows[i] = nr
	}
	return NewTable(cols, rows), nil
}
