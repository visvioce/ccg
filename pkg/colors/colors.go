package colors

// ANSI color codes for terminal output
const (
	Reset        = "\x1B[0m"
	Bold         = "\x1B[1m"
	Dim          = "\x1B[2m"
	Italic       = "\x1B[3m"
	Underline    = "\x1B[4m"
	Blink        = "\x1B[5m"
	Reverse      = "\x1B[7m"
	Hidden       = "\x1B[8m"

	// Foreground colors
	Black   = "\x1B[30m"
	Red     = "\x1B[31m"
	Green   = "\x1B[32m"
	Yellow  = "\x1B[33m"
	Blue    = "\x1B[34m"
	Magenta = "\x1B[35m"
	Cyan    = "\x1B[36m"
	White   = "\x1B[37m"

	// Bright foreground colors
	BrightBlack   = "\x1B[90m"
	BrightRed     = "\x1B[91m"
	BrightGreen   = "\x1B[92m"
	BrightYellow  = "\x1B[93m"
	BrightBlue    = "\x1B[94m"
	BrightMagenta = "\x1B[95m"
	BrightCyan    = "\x1B[96m"
	BrightWhite   = "\x1B[97m"

	// Bold + Color combinations (common in CCR)
	BoldRed     = "\x1B[1m\x1B[31m"
	BoldGreen   = "\x1B[1m\x1B[32m"
	BoldYellow  = "\x1B[1m\x1B[33m"
	BoldBlue    = "\x1B[1m\x1B[34m"
	BoldMagenta = "\x1B[1m\x1B[35m"
	BoldCyan    = "\x1B[1m\x1B[36m"
	BoldWhite   = "\x1B[1m\x1B[37m"
)

// Colorize wraps text with color codes
func Colorize(color, text string) string {
	return color + text + Reset
}

// Success returns green checkmark
func Success(text string) string {
	return Colorize(Green, "✓") + " " + text
}

// Error returns red X
func Error(text string) string {
	return Colorize(Red, "✗") + " " + text
}

// Warning returns yellow warning
func Warning(text string) string {
	return Colorize(Yellow, "⚠") + " " + text
}

// Info returns blue info
func Info(text string) string {
	return Colorize(Blue, "ℹ") + " " + text
}

// BoldText returns bold text
func BoldText(text string) string {
	return Colorize(Bold, text)
}

// DimText returns dim text
func DimText(text string) string {
	return Colorize(Dim, text)
}
