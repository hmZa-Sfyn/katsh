package main

import (
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────
//  Table renderer
// ─────────────────────────────────────────────

// RenderTable formats a slice of Rows into aligned, colorized lines.
func RenderTable(cols []string, rows []Row) []string {
	if len(cols) == 0 {
		return []string{c(ansiGrey, "  (no columns)")}
	}
	if len(rows) == 0 {
		return []string{c(ansiGrey, "  (empty — 0 rows)")}
	}

	// Compute max width per column (header vs data)
	widths := make(map[string]int, len(cols))
	for _, col := range cols {
		widths[col] = len(col)
	}
	for _, row := range rows {
		for _, col := range cols {
			if v := row[col]; len(v) > widths[col] {
				widths[col] = len(v)
			}
		}
	}

	var lines []string

	// ── Header ──
	var hdr strings.Builder
	hdr.WriteString("  ")
	for i, col := range cols {
		label := strings.ToUpper(col)
		hdr.WriteString(ansiBold + ansiCyan + padR(label, widths[col]) + ansiReset)
		if i < len(cols)-1 {
			hdr.WriteString("  ")
		}
	}
	lines = append(lines, hdr.String())

	// ── Separator ──
	var sep strings.Builder
	sep.WriteString("  ")
	for i, col := range cols {
		sep.WriteString(ansiGrey + strings.Repeat("─", widths[col]) + ansiReset)
		if i < len(cols)-1 {
			sep.WriteString("  ")
		}
	}
	lines = append(lines, sep.String())

	// ── Data rows ──
	for ri, row := range rows {
		var ln strings.Builder
		ln.WriteString("  ")
		for i, col := range cols {
			val := row[col]
			color := cellColor(col, val, ri)
			ln.WriteString(color + padR(val, widths[col]) + ansiReset)
			if i < len(cols)-1 {
				ln.WriteString("  ")
			}
		}
		lines = append(lines, ln.String())
	}

	// ── Footer ──
	lines = append(lines, fmt.Sprintf("  %s%d row(s)%s", ansiGrey, len(rows), ansiReset))
	return lines
}

// RenderKV renders key-value pairs as a two-column table.
func RenderKV(pairs [][2]string) []string {
	cols := []string{"key", "value"}
	rows := make([]Row, len(pairs))
	for i, p := range pairs {
		rows[i] = Row{"key": p[0], "value": p[1]}
	}
	return RenderTable(cols, rows)
}

// cellColor returns an ANSI color for a cell based on column semantics and value.
func cellColor(col, val string, rowIdx int) string {
	col = strings.ToLower(col)
	val = strings.ToLower(strings.TrimSpace(val))

	switch col {
	case "name", "key", "file", "filename":
		if strings.HasSuffix(val, "/") {
			return ansiCyan // directories
		}
		// Color by extension
		switch fileExt(val) {
		case ".go":
			return ansiCyan
		case ".sh", ".bash", ".zsh":
			return ansiGreen
		case ".md", ".txt", ".log":
			return ansiWhite
		case ".json", ".yaml", ".yml", ".toml":
			return ansiYellow
		case ".png", ".jpg", ".jpeg", ".gif", ".svg":
			return ansiMagenta
		}
		return ansiWhite

	case "type":
		switch val {
		case "dir":
			return ansiBold + ansiCyan
		case "file":
			return ansiGrey
		case "symlink":
			return ansiMagenta
		}

	case "status", "stat", "state":
		switch {
		case val == "running" || val == "r":
			return ansiGreen
		case val == "sleeping" || val == "s":
			return ansiYellow
		case val == "stopped" || val == "t":
			return ansiRed
		case val == "zombie" || val == "z":
			return ansiBold + ansiRed
		}

	case "cpu", "%cpu":
		if pct := parseFloat(val); pct > 80 {
			return ansiBold + ansiRed
		} else if pct > 40 {
			return ansiYellow
		}
		return ansiGreen

	case "mem", "%mem":
		if pct := parseFloat(val); pct > 50 {
			return ansiRed
		} else if pct > 20 {
			return ansiYellow
		}
		return ansiGreen

	case "use%", "use":
		if pct := parseFloat(strings.TrimSuffix(val, "%")); pct > 85 {
			return ansiBold + ansiRed
		} else if pct > 60 {
			return ansiYellow
		}
		return ansiGreen

	case "perms", "mode":
		return ansiGrey

	case "size":
		return ansiDarkCyan

	case "modified", "created", "updated", "time", "date":
		return ansiDim + ansiWhite

	case "pid":
		return ansiDim + ansiWhite

	case "id":
		return ansiGrey

	case "error", "err":
		if val != "" && val != "0" && val != "ok" {
			return ansiRed
		}
		return ansiGreen
	}

	// Alternating row dim for readability
	if rowIdx%2 == 0 {
		return ansiWhite
	}
	return ansiDim + ansiWhite
}

// padR pads a string to width n with trailing spaces.
func padR(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// fileExt extracts a lowercase file extension.
func fileExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return strings.ToLower(name[idx:])
}

// parseFloat tries to parse a float from a string like "12.3%" → 12.3.
func parseFloat(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
