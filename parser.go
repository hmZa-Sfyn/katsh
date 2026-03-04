package main

import (
	"strings"
)

// ─────────────────────────────────────────────
//  Command line parser
// ─────────────────────────────────────────────
//
// Supported syntax:
//   cmd [args]
//   cmd | select a,b,c
//   cmd | where col=val
//   cmd | where col!=val
//   cmd | where col>val
//   cmd | where col<val
//   cmd | grep text
//   cmd | sort col [asc|desc]
//   cmd | limit N
//   cmd | skip N
//   cmd | count
//   cmd | unique col
//   cmd | reverse
//   cmd | fmt json|csv|tsv
//   cmd #=          (auto-key store)
//   cmd #=mykey     (named store)
//   cmd ## comment  (comment, stripped)

// Parse converts a raw input line into a ParsedCommand.
func Parse(raw string) *ParsedCommand {
	raw = strings.TrimSpace(raw)
	pc := &ParsedCommand{}

	if raw == "" {
		return pc
	}

	// 1. Strip inline comment  ## this is ignored
	if idx := strings.Index(raw, " ##"); idx != -1 {
		pc.Comment = strings.TrimSpace(raw[idx+3:])
		raw = strings.TrimSpace(raw[:idx])
	}

	// 2. Extract box-store operator  #=key  or  #=
	//    Must be at the very end (after all pipes).
	if idx := strings.LastIndex(raw, "#="); idx != -1 {
		// Make sure there's no pipe AFTER the #=
		tail := strings.TrimSpace(raw[idx+2:])
		if !strings.ContainsAny(tail, " \t|") {
			pc.ShouldStore = true
			pc.StoreKey = tail
			raw = strings.TrimSpace(raw[:idx])
		}
	}

	// 3. Background execution suffix  &
	if strings.HasSuffix(raw, " &") || raw == "&" {
		pc.BgRun = true
		raw = strings.TrimSuffix(strings.TrimSpace(raw), "&")
		raw = strings.TrimSpace(raw)
	}

	// 4. Split on pipes
	segments := splitOnPipes(raw)
	if len(segments) == 0 {
		return pc
	}

	// First segment = base command
	pc.Args = tokenize(segments[0])

	// Remaining segments = pipe stages
	for _, seg := range segments[1:] {
		ps := parsePipeStage(strings.TrimSpace(seg))
		if ps.Op != "" {
			pc.Pipes = append(pc.Pipes, ps)
		}
	}

	return pc
}

// parsePipeStage parses one pipe segment like "select name,size" or "where cpu>50".
func parsePipeStage(seg string) PipeStage {
	parts := strings.SplitN(seg, " ", 2)
	op := strings.ToLower(strings.TrimSpace(parts[0]))
	var args []string

	if len(parts) > 1 {
		rest := strings.TrimSpace(parts[1])
		switch op {
		case "select", "cols":
			// Comma-separated column list
			for _, a := range strings.Split(rest, ",") {
				if t := strings.TrimSpace(a); t != "" {
					args = append(args, t)
				}
			}
		case "sort", "orderby", "order":
			// "sort colname [asc|desc]"
			fields := strings.Fields(rest)
			args = fields
		case "fmt", "format":
			args = []string{strings.ToLower(rest)}
		default:
			// where, grep, limit, skip, unique — split on spaces
			args = strings.Fields(rest)
		}
	}

	return PipeStage{Op: op, Args: args}
}

// splitOnPipes splits a command string on | characters,
// respecting single and double quoted strings.
func splitOnPipes(s string) []string {
	var segments []string
	var cur strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch {
		case inQuote:
			cur.WriteRune(ch)
			if ch == quoteChar {
				inQuote = false
			}
		case ch == '"' || ch == '\'':
			inQuote = true
			quoteChar = ch
			cur.WriteRune(ch)
		case ch == '|':
			segments = append(segments, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(ch)
		}
	}
	if cur.Len() > 0 {
		segments = append(segments, cur.String())
	}
	return segments
}

// tokenize splits a command line into tokens, respecting quotes.
func tokenize(s string) []string {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range strings.TrimSpace(s) {
		switch {
		case inQuote:
			if ch == quoteChar {
				inQuote = false
			} else {
				cur.WriteRune(ch)
			}
		case ch == '"' || ch == '\'':
			inQuote = true
			quoteChar = ch
		case ch == ' ' || ch == '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(ch)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}
