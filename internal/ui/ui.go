package ui

import (
	"fmt"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Cyan    = "\033[36m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Red     = "\033[31m"
	Magenta = "\033[35m"
	Blue    = "\033[34m"
	White   = "\033[97m"
)

const separator = 88

// Banner prints the ASCII art banner
func Banner() {
	fmt.Print(Cyan + Bold + `
   ██████╗ ██████╗ ████████╗██╗███████╗ █████╗  ██████╗██╗ ██████╗ ███╗   ██╗███████╗███████╗
  ██╔════╝██╔═══██╗╚══██╔══╝██║╚══███╔╝██╔══██╗██╔════╝██║██╔═══██╗████╗  ██║██╔════╝██╔════╝
  ██║     ██║   ██║   ██║   ██║  ███╔╝ ███████║██║     ██║██║   ██║██╔██╗ ██║█████╗  ███████╗
  ██║     ██║   ██║   ██║   ██║ ███╔╝  ██╔══██║██║     ██║██║   ██║██║╚██╗██║██╔══╝  ╚════██║
  ╚██████╗╚██████╔╝   ██║   ██║███████╗██║  ██║╚██████╗██║╚██████╔╝██║ ╚████║███████╗███████║
   ╚═════╝ ╚═════╝    ╚═╝   ╚═╝╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚══════╝╚══════╝` + Reset + "\n")

	fmt.Printf("%s                          %sUSDT/BOB%s%s · Binance P2P · CriptoYa API%s\n",
		Dim, Magenta, Reset, Dim, Reset)
	fmt.Printf("%s                          %s%s\n",
		Dim, time.Now().Format("02/01/2026 15:04:05"), Reset)
	fmt.Println(Dim + "  " + strings.Repeat("─", separator) + Reset)
}

// StepStart prints a step starting message
func StepStart(step, total int, icon, msg string) {
	fmt.Printf("\n  %s%s[%d/%d]%s %s %s\n", Bold, Blue, step, total, Reset, icon, msg)
}

// Success prints a success message
func Success(msg string) {
	fmt.Printf("        %s%s✓ %s%s\n", Green, Bold, msg, Reset)
}

// Info prints an info detail message
func Info(msg string) {
	fmt.Printf("        %s%s→ %s%s\n", Dim, Cyan, msg, Reset)
}

// Warn prints a warning message
func Warn(msg string) {
	fmt.Printf("        %s%s⚠ %s%s\n", Yellow, Bold, msg, Reset)
}

// Fatal prints a fatal error message
func Fatal(msg string) {
	fmt.Printf("\n  %s%s✗ FATAL: %s%s\n\n", Red, Bold, msg, Reset)
}

// Price prints the cotizacion in a highlighted box
func Price(bid float64) {
	content := fmt.Sprintf("  💵  1 USDT = %.4f BOB  ", bid)
	border := strings.Repeat("━", len(content))

	fmt.Println()
	fmt.Printf("        %s%s┏%s┓%s\n", Bold, Cyan, border, Reset)
	fmt.Printf("        %s%s┃%s%s%s%s┃%s\n",
		Bold, Cyan, White+Bold, content, Reset, Cyan+Bold, Reset)
	fmt.Printf("        %s%s┗%s┛%s\n", Bold, Cyan, border, Reset)
	fmt.Println()
}

// Done prints the final success message
func Done() {
	fmt.Println()
	fmt.Println(Dim + "  " + strings.Repeat("─", separator) + Reset)
	fmt.Printf("\n  %s%s✓ Proceso completado exitosamente%s\n\n", Green, Bold, Reset)
}
