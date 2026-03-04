package main

import (
	"fmt"
	"strings"
)

// ─────────────────────────────────────────────
//  ANSI terminal escape codes
// ─────────────────────────────────────────────

const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiDim        = "\033[2m"
	ansiItalic     = "\033[3m"
	ansiUnderline  = "\033[4m"
	ansiStrike     = "\033[9m"
	ansiBlack      = "\033[30m"
	ansiRed        = "\033[91m"
	ansiGreen      = "\033[92m"
	ansiYellow     = "\033[93m"
	ansiBlue       = "\033[94m"
	ansiMagenta    = "\033[95m"
	ansiCyan       = "\033[96m"
	ansiWhite      = "\033[97m"
	ansiGrey       = "\033[90m"
	ansiDarkGreen  = "\033[32m"
	ansiDarkCyan   = "\033[36m"
	ansiDarkYellow = "\033[33m"
	ansiBgRed      = "\033[41m"
	ansiBgGreen    = "\033[42m"
	ansiBgBlue     = "\033[44m"
	ansiBgGrey     = "\033[100m"
)

// c wraps text in a color and resets after.
func c(color, text string) string {
	return color + text + ansiReset
}

// bold wraps text in bold.
func bold(text string) string {
	return ansiBold + text + ansiReset
}

// dim wraps text in dim.
func dim(text string) string {
	return ansiDim + text + ansiReset
}

// errMsg formats an error line.
func errMsg(msg string) string {
	return fmt.Sprintf("  %s✖ %s%s", ansiRed, msg, ansiReset)
}

// okMsg formats a success line.
func okMsg(msg string) string {
	return fmt.Sprintf("  %s✔ %s%s", ansiGreen, msg, ansiReset)
}

// infoMsg formats an info line.
func infoMsg(msg string) string {
	return fmt.Sprintf("  %s· %s%s", ansiCyan, msg, ansiReset)
}

// warnMsg formats a warning line.
func warnMsg(msg string) string {
	return fmt.Sprintf("  %s⚠ %s%s", ansiYellow, msg, ansiReset)
}

// boxMsg formats a box-store notification.
func boxMsg(key string, id int, size string) string {
	return fmt.Sprintf("  %s📦 box[%s\"%s\"%s] id:%s%d%s  %s%s%s",
		ansiYellow,
		ansiBold, key, ansiReset+ansiYellow,
		ansiBold, id, ansiReset+ansiYellow,
		ansiDim, size, ansiReset,
	)
}

// sectionHeader prints a styled section title.
func sectionHeader(title string) string {
	line := strings.Repeat("─", 50)
	return fmt.Sprintf("\n  %s%s %s %s%s\n",
		ansiBold+ansiCyan, "◈", title, line[:localMax(0, 46-len(title))], ansiReset)
}

func localMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// banner returns the startup ASCII art.
func banner() string {
	return c(ansiGreen, `
  ╔══════════════════════════════════════════════════════╗
  ║  StructSH  ·  Structured Shell  ·  v0.2.0           ║
  ║  Everything is data. Every output is a table.       ║
  ╚══════════════════════════════════════════════════════╝`) +
		"\n" + c(ansiGrey, "  Type 'help' to get started. Ctrl-D or 'exit' to quit.\n")
}

// renderPrompt builds the shell prompt string.
func renderPrompt(cwd, user string, exitCode int) string {
	short := strings.Replace(cwd, homeDir(), "~", 1)
	// Color the arrow red if last command failed
	arrowColor := ansiGreen
	if exitCode != 0 {
		arrowColor = ansiRed
	}
	return fmt.Sprintf("%s%s%s %s❯%s ",
		ansiBold+ansiCyan, short, ansiReset,
		arrowColor, ansiReset,
	)
}
