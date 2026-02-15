package ui

import (
	"fmt"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Cyan      = "\033[36m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Red       = "\033[31m"
	Magenta   = "\033[35m"
	Blue      = "\033[34m"
	White     = "\033[97m"
	BgCyan    = "\033[46m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
)

// Banner prints the ASCII art banner
func Banner() {
	banner := `
` + Cyan + Bold + `
   ██████╗ ██████╗ ████████╗██╗███████╗ █████╗  ██████╗██╗ ██████╗ ███╗   ██╗███████╗███████╗
  ██╔════╝██╔═══██╗╚══██╔══╝██║╚══███╔╝██╔══██╗██╔════╝██║██╔═══██╗████╗  ██║██╔════╝██╔════╝
  ██║     ██║   ██║   ██║   ██║  ███╔╝ ███████║██║     ██║██║   ██║██╔██╗ ██║█████╗  ███████╗
  ██║     ██║   ██║   ██║   ██║ ███╔╝  ██╔══██║██║     ██║██║   ██║██║╚██╗██║██╔══╝  ╚════██║
  ╚██████╗╚██████╔╝   ██║   ██║███████╗██║  ██║╚██████╗██║╚██████╔╝██║ ╚████║███████╗███████║
   ╚═════╝ ╚═════╝    ╚═╝   ╚═╝╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝╚══════╝` + Reset + `
` + Dim + `                          ` + Magenta + `USDT/BOB` + Reset + Dim + ` · Binance P2P · CriptoYa API` + Reset + `
` + Dim + `                          ` + formatTimestamp() + Reset + `
`
	fmt.Print(banner)
	fmt.Println(Dim + "  " + strings.Repeat("─", 88) + Reset)
}

func formatTimestamp() string {
	return time.Now().Format("02/01/2006 15:04:05")
}

// StepStart prints a step starting message
func StepStart(step int, icon string, msg string) {
	fmt.Printf("\n  %s%s [%d/5]%s %s %s\n", Bold, Blue, step, Reset, icon, msg)
}

// Success prints a success message
func Success(msg string) {
	fmt.Printf("        %s%s ✓ %s%s\n", Green, Bold, msg, Reset)
}

// Info prints an info message
func Info(msg string) {
	fmt.Printf("        %s%s→ %s%s\n", Dim, Cyan, msg, Reset)
}

// Warn prints a warning message
func Warn(msg string) {
	fmt.Printf("        %s%s⚠ %s%s\n", Yellow, Bold, msg, Reset)
}

// Error prints an error message
func Error(msg string) {
	fmt.Printf("        %s%s✗ %s%s\n", Red, Bold, msg, Reset)
}

// Fatal prints an error and exits
func Fatal(msg string) {
	fmt.Printf("\n  %s%s ✗ FATAL: %s%s\n\n", Red, Bold, msg, Reset)
}

// Price prints the cotizacion in a highlighted box
func Price(bid float64) {
	line := fmt.Sprintf("  💵 1 USDT = %.4f BOB  ", bid)
	width := len(line) + 4
	border := strings.Repeat("━", width)

	fmt.Println()
	fmt.Printf("        %s%s┏%s┓%s\n", Bold, Cyan, border, Reset)
	fmt.Printf("        %s%s┃  %s%s%s%.4f BOB%s  %s┃%s\n",
		Bold, Cyan,
		White, Bold, "  💵  1 USDT = ", bid, Reset,
		Cyan+Bold, Reset)
	fmt.Printf("        %s%s┗%s┛%s\n", Bold, Cyan, border, Reset)
	fmt.Println()
}

// Done prints the final success message
func Done() {
	fmt.Println()
	fmt.Println(Dim + "  " + strings.Repeat("─", 88) + Reset)
	fmt.Printf("\n  %s%s ✓ Proceso completado exitosamente%s\n\n", Green, Bold, Reset)
}
