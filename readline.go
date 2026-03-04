package main

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// ─────────────────────────────────────────────────────────────────────────────
//  Readline — full raw-mode line editor
//
//  Features:
//   • ↑↓  history navigation
//   • ←→  cursor movement (character)
//   • Ctrl-A/E  home / end
//   • Ctrl-W    delete previous word
//   • Ctrl-U    clear whole line
//   • Ctrl-K    kill to end of line
//   • Ctrl-L    clear screen
//   • Ctrl-C    cancel line
//   • Ctrl-D    EOF / exit when line empty
//   • Tab       completion (commands, paths, $vars, box keys)
//   • Live syntax highlighting while you type
// ─────────────────────────────────────────────────────────────────────────────

// Readline reads one edited line. Returns (line, eof).
func (sh *Shell) Readline(prompt string) (string, bool) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// fallback — plain read
		fmt.Print(prompt)
		var line string
		fmt.Scanln(&line)
		return line, false
	}
	defer term.Restore(fd, oldState)

	fmt.Print(prompt)

	var buf     []rune
	cursor      := 0
	histIdx     := -1
	savedLine   := ""

	// Redraw current line with syntax highlighting
	redraw := func() {
		pLen := visibleLen(prompt)
		// Go to start of this line, jump past prompt, clear rest
		fmt.Printf("\r\033[%dC\033[K", pLen)
		fmt.Print(highlightInput(string(buf)))
		// Reposition cursor
		col := pLen + runeVisualWidth(buf[:cursor])
		fmt.Printf("\r\033[%dC", col)
	}

	for {
		raw := make([]byte, 16)
		n, err := os.Stdin.Read(raw)
		if err != nil || n == 0 {
			return string(buf), true
		}
		b := raw[:n]

		// ── Escape / CSI sequences ───────────────
		if b[0] == 0x1b {
			if n >= 3 && b[1] == '[' {
				switch b[2] {
				case 'A': // ↑ up — history prev
					if len(sh.history) == 0 { continue }
					if histIdx == -1 {
						savedLine = string(buf)
						histIdx = len(sh.history) - 1
					} else if histIdx > 0 {
						histIdx--
					}
					buf = []rune(sh.history[histIdx].Raw)
					cursor = len(buf)
					redraw()

				case 'B': // ↓ down — history next
					if histIdx == -1 { continue }
					if histIdx < len(sh.history)-1 {
						histIdx++
						buf = []rune(sh.history[histIdx].Raw)
					} else {
						histIdx = -1
						buf = []rune(savedLine)
					}
					cursor = len(buf)
					redraw()

				case 'C': // → right
					if cursor < len(buf) { cursor++; redraw() }

				case 'D': // ← left
					if cursor > 0 { cursor--; redraw() }

				case 'H': // Home
					cursor = 0; redraw()

				case 'F': // End
					cursor = len(buf); redraw()

				case '3': // Delete key (ESC [ 3 ~)
					if n >= 4 && b[3] == '~' && cursor < len(buf) {
						buf = append(buf[:cursor], buf[cursor+1:]...)
						redraw()
					}

				case '1': // ESC [ 1 ~ = Home
					if n >= 4 && b[3] == '~' { cursor = 0; redraw() }
				case '4': // ESC [ 4 ~ = End
					if n >= 4 && b[3] == '~' { cursor = len(buf); redraw() }
				}
			}
			continue
		}

		// ── Control characters ───────────────────
		switch b[0] {
		case 0x0d, 0x0a: // Enter
			fmt.Print("\r\n")
			return string(buf), false

		case 0x7f, 0x08: // Backspace
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
				redraw()
			}

		case 0x01: // Ctrl-A — home
			cursor = 0; redraw()

		case 0x05: // Ctrl-E — end
			cursor = len(buf); redraw()

		case 0x0b: // Ctrl-K — kill to end
			buf = buf[:cursor]; redraw()

		case 0x15: // Ctrl-U — kill whole line
			buf = buf[:0]; cursor = 0; redraw()

		case 0x17: // Ctrl-W — delete prev word
			end := cursor
			for cursor > 0 && buf[cursor-1] == ' ' { cursor-- }
			for cursor > 0 && buf[cursor-1] != ' ' { cursor-- }
			buf = append(buf[:cursor], buf[end:]...)
			redraw()

		case 0x0c: // Ctrl-L — clear screen
			fmt.Print("\033[2J\033[H")
			fmt.Print(prompt)
			fmt.Print(highlightInput(string(buf)))
			col := visibleLen(prompt) + runeVisualWidth(buf[:cursor])
			fmt.Printf("\r\033[%dC", col)

		case 0x04: // Ctrl-D — EOF if empty
			if len(buf) == 0 {
				fmt.Print("\r\n")
				return "", true
			}

		case 0x03: // Ctrl-C — cancel line
			fmt.Print("^C\r\n")
			buf = buf[:0]; cursor = 0; histIdx = -1
			fmt.Print(prompt)

		case 0x09: // Tab — completion
			opts := sh.completionOptions(string(buf), cursor)
			switch len(opts) {
			case 0:
				// nothing
			case 1:
				// complete the word
				word := lastWord(string(buf[:cursor]))
				rest := string(buf[cursor:])
				completed := string(buf[:cursor-len([]rune(word))]) + opts[0]
				if !strings.HasSuffix(opts[0], "/") {
					completed += " "
				}
				buf = []rune(completed + rest)
				cursor = len([]rune(completed))
				redraw()
			default:
				// show options below
				fmt.Print("\r\n")
				for i, o := range opts {
					fmt.Printf("  %s%s%s", ansiCyan, o, ansiReset)
					if i < len(opts)-1 { fmt.Print("  ") }
				}
				fmt.Print("\r\n")
				fmt.Print(prompt)
				fmt.Print(highlightInput(string(buf)))
				col := visibleLen(prompt) + runeVisualWidth(buf[:cursor])
				fmt.Printf("\r\033[%dC", col)
			}

		default:
			// Printable / multibyte UTF-8
			ch, size := utf8.DecodeRune(b[:n])
			if ch != utf8.RuneError || size > 1 {
				newBuf := make([]rune, len(buf)+1)
				copy(newBuf, buf[:cursor])
				newBuf[cursor] = ch
				copy(newBuf[cursor+1:], buf[cursor:])
				buf = newBuf
				cursor++
				redraw()
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Live syntax highlighting
// ─────────────────────────────────────────────────────────────────────────────

// highlightInput colorizes a command line string for display.
func highlightInput(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	spans := spanize(line)
	var out strings.Builder
	cmdPos := 0 // which non-space token are we on

	for _, sp := range spans {
		if sp.space {
			out.WriteString(sp.text)
			continue
		}
		t := sp.text

		switch {
		// Comments
		case strings.HasPrefix(t, "##") || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "# ") || t == "#":
			out.WriteString(ansiGrey + t + ansiReset)
			continue

		// Backtick subshell
		case strings.HasPrefix(t, "`"):
			out.WriteString(ansiMagenta + t + ansiReset)
			cmdPos++
			continue

		// String literals
		case len(t) >= 2 && ((t[0] == '"' && t[len(t)-1] == '"') || (t[0] == '\'' && t[len(t)-1] == '\'')):
			out.WriteString(ansiYellow + t + ansiReset)
			cmdPos++
			continue

		// Box-store operator
		case strings.HasPrefix(t, "#="):
			out.WriteString(ansiYellow + t + ansiReset)
			continue

		// Variable reference
		case strings.HasPrefix(t, "$"):
			out.WriteString(ansiDarkCyan + t + ansiReset)
			cmdPos++
			continue

		// Flags
		case strings.HasPrefix(t, "-") && len(t) > 1:
			out.WriteString(ansiCyan + t + ansiReset)
			continue

		// Numbers
		case isNumericStr(t):
			out.WriteString(ansiDarkYellow + t + ansiReset)
			cmdPos++
			continue

		// Operators
		case isSyntaxOp(t):
			out.WriteString(ansiGrey + t + ansiReset)
			continue

		// Keywords
		case isKeyword(t):
			out.WriteString(ansiBold + ansiMagenta + t + ansiReset)
			cmdPos++
			continue
		}

		// First token (command name) and first token after |
		if cmdPos == 0 || sp.afterPipe {
			if isBuiltin(t) {
				out.WriteString(ansiBold + ansiGreen + t + ansiReset)
			} else if findInPath(t) != "" {
				out.WriteString(ansiGreen + t + ansiReset)
			} else {
				// Unknown command — red underline
				out.WriteString(ansiRed + ansiUnderline + t + ansiReset)
			}
		} else {
			out.WriteString(ansiWhite + t + ansiReset)
		}
		cmdPos++
	}
	return out.String()
}

type span struct {
	text      string
	space     bool
	afterPipe bool
}

func spanize(s string) []span {
	var spans []span
	var cur strings.Builder
	inQuote := false
	quoteChar := rune(0)
	inBacktick := false
	prevWasPipe := false

	flush := func(isSpace bool) {
		if cur.Len() == 0 { return }
		t := cur.String()
		spans = append(spans, span{text: t, space: isSpace, afterPipe: prevWasPipe && !isSpace})
		if !isSpace { prevWasPipe = (t == "|") }
		cur.Reset()
	}

	for _, ch := range s {
		switch {
		case inBacktick:
			cur.WriteRune(ch)
			if ch == '`' { inBacktick = false; flush(false) }
		case inQuote:
			cur.WriteRune(ch)
			if ch == quoteChar { inQuote = false; flush(false) }
		case ch == '`':
			flush(false)
			inBacktick = true
			cur.WriteRune(ch)
		case ch == '"' || ch == '\'':
			flush(false)
			inQuote = true
			quoteChar = ch
			cur.WriteRune(ch)
		case ch == ' ' || ch == '\t':
			flush(false)
			cur.WriteRune(ch)
			flush(true)
		default:
			cur.WriteRune(ch)
		}
	}
	flush(false)
	return spans
}

func isKeyword(t string) bool {
	switch strings.ToLower(t) {
	case "if", "elif", "else", "fi", "for", "while", "do", "done",
		"func", "return", "in", "range", "break", "continue",
		"and", "or", "not", "true", "false", "null", "print":
		return true
	}
	return false
}

func isSyntaxOp(t string) bool {
	switch t {
	case "|", "||", "&&", "=", "==", "!=", ">=", "<=", ">", "<",
		"++", "--", "+=", "-=", "*=", "/=", "%=",
		"{", "}", "(", ")", "[", "]", ":", ";", "->", "=>", ".":
		return true
	}
	return false
}

func isNumericStr(t string) bool {
	if t == "" { return false }
	dot := false
	for i, ch := range t {
		if i == 0 && (ch == '-' || ch == '+') { continue }
		if ch == '.' { if dot { return false }; dot = true; continue }
		if ch < '0' || ch > '9' { return false }
	}
	return true
}

// ─────────────────────────────────────────────────────────────────────────────
//  Tab completion
// ─────────────────────────────────────────────────────────────────────────────

func (sh *Shell) completionOptions(line string, cursor int) []string {
	prefix := line[:cursor]
	word   := lastWord(prefix)
	toks   := strings.Fields(prefix)
	isCmd  := len(toks) == 0 || (len(toks) == 1 && !strings.HasSuffix(prefix, " "))

	var opts []string

	if isCmd {
		for _, b := range allBuiltinNames() {
			if strings.HasPrefix(b, word) { opts = append(opts, b) }
		}
		for name := range sh.aliases {
			if strings.HasPrefix(name, word) { opts = append(opts, name) }
		}
	} else if strings.HasPrefix(word, "$") {
		pfx := word[1:]
		for k := range sh.vars {
			if strings.HasPrefix(k, pfx) { opts = append(opts, "$"+k) }
		}
	} else {
		// Path completion
		dir := sh.cwd
		filePfx := word
		if idx := strings.LastIndex(word, "/"); idx >= 0 {
			dir = resolvePath(sh.cwd, word[:idx+1])
			filePfx = word[idx+1:]
		}
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, filePfx) {
					pfxBase := word[:len(word)-len(filePfx)]
					if e.IsDir() { name += "/" }
					opts = append(opts, pfxBase+name)
				}
			}
		}
		// Box key completion after "box get/rm/rename/tag"
		if len(toks) >= 2 && toks[0] == "box" {
			for _, k := range sh.box.Keys() {
				if strings.HasPrefix(k, word) { opts = append(opts, k) }
			}
		}
	}
	return opts
}

func lastWord(s string) string {
	if strings.HasSuffix(s, " ") { return "" }
	parts := strings.Fields(s)
	if len(parts) == 0 { return "" }
	return parts[len(parts)-1]
}

func allBuiltinNames() []string {
	return []string{
		"cd","pwd","pushd","popd","dirs",
		"ls","ll","la","tree","du","df",
		"cat","head","tail","touch","mkdir","rmdir","rm","cp","mv","ln",
		"wc","stat","file","find","diff",
		"grep","sed","awk","cut","tr","sort","uniq","tee","split","xargs",
		"chmod","chown",
		"ps","kill","sleep","jobs",
		"uname","uptime","date","cal","hostname","whoami","id","groups","who","w",
		"ping","curl","wget","nslookup","dig","ifconfig","ip",
		"md5sum","sha1sum","sha256sum",
		"tar","gzip","gunzip","zip","unzip",
		"echo","printf","yes","seq","base64","rev",
		"set","unset","vars","export","env","printenv",
		"alias","unalias","aliases","which","type",
		"bc","factor","random",
		"box","history","clear","help","man","exit","quit","source","watch",
		"if","for","while","func","print","return",
	}
}

// ─────────────────────────────────────────────────────────────────────────────
//  Visual width helpers
// ─────────────────────────────────────────────────────────────────────────────

// visibleLen returns terminal column width of a string, skipping ANSI escapes.
func visibleLen(s string) int {
	n := 0
	esc := false
	for _, ch := range s {
		if esc { if ch == 'm' { esc = false }; continue }
		if ch == '\033' { esc = true; continue }
		n++
	}
	return n
}

// runeVisualWidth returns display width of a rune slice (approx 1 per rune).
func runeVisualWidth(r []rune) int { return len(r) }
