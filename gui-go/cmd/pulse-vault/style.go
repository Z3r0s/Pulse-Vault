package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Pulse-cyan console chrome. Colors only when stdout is a TTY and the user
// has not asked for plain output (NO_COLOR / PULSEVAULT_PLAIN / --plain).
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiCyan   = "\x1b[38;5;50m" // pulse cyan
	ansiTeal   = "\x1b[38;5;44m"
	ansiFg     = "\x1b[38;5;255m"
	ansiMuted  = "\x1b[38;5;245m"
	ansiRed    = "\x1b[38;5;204m"
	ansiYellow = "\x1b[38;5;221m"
	ansiGreen  = "\x1b[38;5;84m"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

var (
	plainMode   bool
	colorActive bool
)

func initTheme() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("PULSEVAULT_PLAIN") == "1" || plainMode {
		colorActive = false
		return
	}
	colorActive = stdoutIsTTY()
	if colorActive {
		enableVT()
	}
}

func stdoutIsTTY() bool {
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func stdinIsTTY() bool {
	st, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !colorActive || s == "" {
		return s
	}
	return code + s + ansiReset
}

func bold(s string) string   { return paint(ansiBold, s) }
func cyan(s string) string   { return paint(ansiCyan, s) }
func fg(s string) string     { return paint(ansiFg, s) }
func muted(s string) string  { return paint(ansiMuted, s) }
func red(s string) string    { return paint(ansiRed, s) }
func yellow(s string) string { return paint(ansiYellow, s) }
func green(s string) string  { return paint(ansiGreen, s) }

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func visLen(s string) int { return utf8.RuneCountInString(stripANSI(s)) }

func padRight(s string, width int) string {
	n := visLen(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func truncate(s string, width int) string {
	plain := stripANSI(s)
	if utf8.RuneCountInString(plain) <= width {
		return s
	}
	r := []rune(plain)
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}

func humanSize(n int64) string {
	if n < 0 {
		n = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[0])
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func box(title string, lines []string, width int) string {
	if width < 24 {
		width = 24
	}
	inner := width - 2
	var b strings.Builder
	title = strings.TrimSpace(title)
	top := "╭"
	if title != "" {
		label := "─ " + title + " "
		pad := inner - visLen(label)
		if pad < 0 {
			label = truncate(label, inner)
			pad = inner - visLen(label)
		}
		top += label + strings.Repeat("─", pad)
	} else {
		top += strings.Repeat("─", inner)
	}
	top += "╮"
	b.WriteString(cyan(top))
	b.WriteByte('\n')
	for _, line := range lines {
		body := truncate(line, inner-1)
		b.WriteString(cyan("│"))
		b.WriteString(padRight(" "+body, inner))
		b.WriteString(cyan("│"))
		b.WriteByte('\n')
	}
	b.WriteString(cyan("╰" + strings.Repeat("─", inner) + "╯"))
	return b.String()
}

func kv(key, value string) string {
	return muted(padRight(key, 12)) + fg(value)
}

func markOK() string   { return green("✓") }
func markWarn() string { return yellow("!") }
func markErr() string  { return red("✗") }
func markLock() string { return red("●") }
func markOpen() string { return cyan("●") }
func glyph() string    { return cyan("⬡") }
