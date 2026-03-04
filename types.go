package main

import "time"

// ─────────────────────────────────────────────
//  Core data types shared across all files
// ─────────────────────────────────────────────

// Row is a single structured record: column name → value.
type Row map[string]string

// ValueType describes what kind of data a Box Entry holds.
type ValueType string

const (
	TypeTable ValueType = "table"
	TypeText  ValueType = "text"
)

// Result is the structured output of any command execution.
// Commands either produce a Table (rows + cols) or raw Text.
type Result struct {
	Cols    []string // ordered column names
	Rows    []Row    // data rows (when IsTable == true)
	Text    string   // raw text output (when IsTable == false)
	IsTable bool
}

// NewTable creates a table Result.
func NewTable(cols []string, rows []Row) *Result {
	return &Result{Cols: cols, Rows: rows, IsTable: true}
}

// NewText creates a plain-text Result.
func NewText(text string) *Result {
	return &Result{Text: text, IsTable: false}
}

// BoxEntry is one item stored in the Box.
type BoxEntry struct {
	ID      int
	Key     string
	Type    ValueType
	Rows    []Row    // when Type == TypeTable
	Cols    []string // ordered col names for table
	Text    string   // when Type == TypeText
	Source  string   // the raw command that produced this
	Tags    []string // user-defined tags
	Created time.Time
	Updated time.Time
}

// Size returns a human-readable size hint.
func (e *BoxEntry) Size() string {
	switch e.Type {
	case TypeTable:
		return formatInt(len(e.Rows)) + " rows"
	case TypeText:
		return formatInt(len(e.Text)) + " bytes"
	default:
		return "-"
	}
}

// PipeStage is one pipe segment: op + args.
// e.g.  | select name,size   → {Op:"select", Args:["name","size"]}
type PipeStage struct {
	Op   string
	Args []string
}

// ParsedCommand is the full result of parsing one input line.
type ParsedCommand struct {
	Args        []string    // base command + its flags
	Pipes       []PipeStage // chained pipe transforms
	StoreKey    string      // box key (empty = auto)
	ShouldStore bool        // whether #= was present
	BgRun       bool        // whether & suffix was present (future use)
	Comment     string      // trailing ## comment, stripped
}

// Alias maps a short name to an expanded command string.
type Alias struct {
	Name    string
	Expand  string
	Created time.Time
}

// formatInt just converts an int to a string.
func formatInt(n int) string {
	s := ""
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
