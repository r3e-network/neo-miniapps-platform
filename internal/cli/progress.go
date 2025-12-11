// Package cli provides CLI utilities including progress bars and colored output
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// ProgressBar represents a progress bar
type ProgressBar struct {
	total       int
	current     int
	width       int
	prefix      string
	mu          sync.Mutex
	writer      io.Writer
	startTime   time.Time
	showPercent bool
	showTime    bool
	colorize    bool
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int, prefix string) *ProgressBar {
	return &ProgressBar{
		total:       total,
		current:     0,
		width:       50,
		prefix:      prefix,
		writer:      os.Stdout,
		startTime:   time.Now(),
		showPercent: true,
		showTime:    true,
		colorize:    isTerminal(),
	}
}

// SetWidth sets the width of the progress bar
func (pb *ProgressBar) SetWidth(width int) *ProgressBar {
	pb.width = width
	return pb
}

// SetWriter sets the output writer
func (pb *ProgressBar) SetWriter(w io.Writer) *ProgressBar {
	pb.writer = w
	return pb
}

// DisableColor disables colored output
func (pb *ProgressBar) DisableColor() *ProgressBar {
	pb.colorize = false
	return pb
}

// Increment increments the progress bar by 1
func (pb *ProgressBar) Increment() {
	pb.Add(1)
}

// Add adds n to the progress bar
func (pb *ProgressBar) Add(n int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current += n
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.render()
}

// Set sets the current value
func (pb *ProgressBar) Set(current int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current = current
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.render()
}

// Finish completes the progress bar
func (pb *ProgressBar) Finish() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current = pb.total
	pb.render()
	fmt.Fprintln(pb.writer)
}

// render renders the progress bar
func (pb *ProgressBar) render() {
	percent := float64(pb.current) / float64(pb.total)
	filled := int(float64(pb.width) * percent)

	// Build progress bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)

	// Add color if enabled
	if pb.colorize {
		if percent < 0.5 {
			bar = ColorYellow + bar + ColorReset
		} else if percent < 1.0 {
			bar = ColorCyan + bar + ColorReset
		} else {
			bar = ColorGreen + bar + ColorReset
		}
	}

	// Build output string
	output := fmt.Sprintf("\r%s [%s]", pb.prefix, bar)

	if pb.showPercent {
		output += fmt.Sprintf(" %.1f%%", percent*100)
	}

	if pb.showTime && pb.current > 0 {
		elapsed := time.Since(pb.startTime)
		remaining := time.Duration(float64(elapsed) / percent * (1 - percent))
		output += fmt.Sprintf(" | %s elapsed | %s remaining", formatDuration(elapsed), formatDuration(remaining))
	}

	fmt.Fprint(pb.writer, output)
}

// Spinner represents a loading spinner
type Spinner struct {
	frames   []string
	current  int
	prefix   string
	suffix   string
	mu       sync.Mutex
	writer   io.Writer
	active   bool
	colorize bool
	done     chan bool
}

// NewSpinner creates a new spinner
func NewSpinner(prefix string) *Spinner {
	return &Spinner{
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		current:  0,
		prefix:   prefix,
		writer:   os.Stdout,
		active:   false,
		colorize: isTerminal(),
		done:     make(chan bool),
	}
}

// SetSuffix sets the suffix text
func (s *Spinner) SetSuffix(suffix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suffix = suffix
}

// Start starts the spinner
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	s.active = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				if !s.active {
					s.mu.Unlock()
					return
				}
				s.render()
				s.current = (s.current + 1) % len(s.frames)
				s.mu.Unlock()
			case <-s.done:
				return
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.done)

	// Clear the line
	fmt.Fprint(s.writer, "\r"+strings.Repeat(" ", 80)+"\r")
}

// Success stops the spinner and shows a success message
func (s *Spinner) Success(message string) {
	s.Stop()
	if s.colorize {
		fmt.Fprintf(s.writer, "%s✓%s %s\n", ColorGreen, ColorReset, message)
	} else {
		fmt.Fprintf(s.writer, "✓ %s\n", message)
	}
}

// Error stops the spinner and shows an error message
func (s *Spinner) Error(message string) {
	s.Stop()
	if s.colorize {
		fmt.Fprintf(s.writer, "%s✗%s %s\n", ColorRed, ColorReset, message)
	} else {
		fmt.Fprintf(s.writer, "✗ %s\n", message)
	}
}

// render renders the spinner
func (s *Spinner) render() {
	frame := s.frames[s.current]
	if s.colorize {
		frame = ColorCyan + frame + ColorReset
	}

	output := fmt.Sprintf("\r%s %s", frame, s.prefix)
	if s.suffix != "" {
		output += " " + s.suffix
	}

	fmt.Fprint(s.writer, output)
}

// Helper functions

// Colorize returns a colored string
func Colorize(text string, color string) string {
	if !isTerminal() {
		return text
	}
	return color + text + ColorReset
}

// Success prints a success message
func Success(message string) {
	if isTerminal() {
		fmt.Printf("%s✓%s %s\n", ColorGreen, ColorReset, message)
	} else {
		fmt.Printf("✓ %s\n", message)
	}
}

// Error prints an error message
func Error(message string) {
	if isTerminal() {
		fmt.Printf("%s✗%s %s\n", ColorRed, ColorReset, message)
	} else {
		fmt.Printf("✗ %s\n", message)
	}
}

// Warning prints a warning message
func Warning(message string) {
	if isTerminal() {
		fmt.Printf("%s⚠%s %s\n", ColorYellow, ColorReset, message)
	} else {
		fmt.Printf("⚠ %s\n", message)
	}
}

// Info prints an info message
func Info(message string) {
	if isTerminal() {
		fmt.Printf("%sℹ%s %s\n", ColorBlue, ColorReset, message)
	} else {
		fmt.Printf("ℹ %s\n", message)
	}
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
