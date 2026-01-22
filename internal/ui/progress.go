package ui

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"
)

// flushStdin discards any pending input from stdin to prevent
// terminal response sequences (like cursor position reports, focus events)
// from corrupting the output. Uses timeout-based flushing to catch
// asynchronous terminal responses.
func flushStdin() {
	FlushStdinWithTimeout(30 * time.Millisecond)
}

// Spinner is a tiny terminal spinner helper.
type Spinner struct {
    frames []rune
    idx    int
    out    io.Writer
    colors *ColorConfig
    prefix string
    delay  time.Duration
}

func NewSpinner(out io.Writer, prefix string) *Spinner {
    if out == nil { out = io.Discard }
    return &Spinner{
        frames: []rune{'⠋','⠙','⠹','⠸','⠼','⠴','⠦','⠧','⠇','⠏'},
        idx: 0,
        out: out,
        colors: NewColorConfig(),
        prefix: prefix,
        delay: 120 * time.Millisecond,
    }
}

func (s *Spinner) SetDelay(d time.Duration) { if d > 0 { s.delay = d } }

// Tick renders the next frame with prefix. Caller controls timing via time.Ticker.
func (s *Spinner) Tick() {
	if s.out == nil {
		return
	}
	frame := s.frames[s.idx%len(s.frames)]
	s.idx++
	msg := s.prefix
	if s.colors.Enabled {
		fmt.Fprintf(s.out, "\r%s %c", msg, frame)
	} else {
		fmt.Fprintf(s.out, "\r%s", msg)
	}
}

// ProgressBar renders a terminal progress bar with download statistics.
type ProgressBar struct {
	out        io.Writer
	total      int64
	current    int64
	startTime  time.Time
	lastUpdate time.Time
	isTTY      bool
	lastPct    float64 // for non-TTY threshold updates
	colors     *ColorConfig
	indent     string // indentation prefix (default "  ")
}

// NewProgressBar creates a new progress bar for tracking download progress.
// If total is <= 0, the progress bar will show bytes downloaded without percentage.
func NewProgressBar(out io.Writer, total int64) *ProgressBar {
	if out == nil {
		out = os.Stdout
	}

	// Check if output is a TTY
	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	// Disable terminal focus reporting to prevent ^[[I/^[[O sequences
	// and flush any pending terminal responses that could corrupt output
	if isTTY {
		// Disable focus reporting (CSI ? 1004 l)
		fmt.Fprint(out, "\033[?1004l")
		// Small delay to allow any pending terminal responses to arrive
		time.Sleep(10 * time.Millisecond)
		// Flush any pending terminal responses from stdin
		flushStdin()
	}

	return &ProgressBar{
		out:        out,
		total:      total,
		current:    0,
		startTime:  time.Now(),
		lastUpdate: time.Time{},
		isTTY:      isTTY,
		lastPct:    -1,
		colors:     NewColorConfig(),
		indent:     "  ", // default 2-space indent
	}
}

// SetIndent sets the indentation prefix for the progress bar output.
func (p *ProgressBar) SetIndent(indent string) {
	p.indent = indent
}

// Update updates the progress bar with the current byte count.
func (p *ProgressBar) Update(current int64) {
	p.current = current

	// Rate limit updates to avoid flicker (max 10/sec for TTY)
	now := time.Now()
	if p.isTTY && now.Sub(p.lastUpdate) < 100*time.Millisecond {
		return
	}
	p.lastUpdate = now

	if p.total <= 0 {
		// Unknown total: just show bytes downloaded
		fmt.Fprintf(p.out, "\r%sDownloading... %s", p.indent, FormatBytes(current))
		return
	}

	pct := float64(current) / float64(p.total) * 100

	if p.isTTY {
		p.renderTTY(pct)
	} else {
		// Non-TTY: print at 10% intervals
		threshold := float64(int(pct/10) * 10)
		if threshold > p.lastPct {
			p.lastPct = threshold
			fmt.Fprintf(p.out, "%sDownloading... %.0f%%\n", p.indent, threshold)
		}
	}
}

// renderTTY renders the progress bar for TTY output.
func (p *ProgressBar) renderTTY(pct float64) {
	// Calculate speed
	elapsed := time.Since(p.startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(p.current) / elapsed
	}

	// Calculate ETA
	eta := ""
	if speed > 0 && p.current < p.total {
		remaining := float64(p.total - p.current)
		etaSeconds := remaining / speed
		eta = formatDuration(etaSeconds)
	} else if p.current >= p.total {
		eta = "0s"
	}

	// Get terminal width, default to 80
	width := 80
	if f, ok := p.out.(*os.File); ok {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			width = w
		}
	}

	// Calculate bar width (leave space for stats)
	// Format: "<indent>[████░░░░] 100.0%  999.9GB/999.9GB  999.9MB/s  ETA 99m59s"
	// Approx: indent + 2 + bar + 2 + 7 + 2 + 17 + 2 + 10 + 2 + 10 = ~54 + indent chars for stats
	barWidth := width - 56 - len(p.indent)
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 40 {
		barWidth = 40
	}

	// Build progress bar
	filled := int(pct / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}

	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	// Format output with ANSI escape to clear rest of line (fixes /s/s bug)
	// \033[K clears from cursor to end of line
	fmt.Fprintf(p.out, "\r%s[%s] %5.1f%%   %s/%s   %s   ETA %s\033[K",
		p.indent,
		bar,
		pct,
		FormatBytes(p.current),
		FormatBytes(p.total),
		FormatSpeed(speed),
		eta,
	)
}

// formatDuration formats seconds into a human-readable duration string.
func formatDuration(seconds float64) string {
	if seconds < 0 {
		return "--"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		mins := int(seconds) / 60
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(seconds) / 3600
	mins := (int(seconds) % 3600) / 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// Finish completes the progress bar and moves to the next line.
func (p *ProgressBar) Finish() {
	if p.isTTY {
		// Final update to show 100%
		if p.total > 0 {
			p.renderTTY(100)
		}
		fmt.Fprintln(p.out)
		// Flush any pending terminal responses that accumulated during progress updates
		flushStdin()
	} else if p.total > 0 && p.lastPct < 100 {
		// Ensure we print 100% for non-TTY
		fmt.Fprintf(p.out, "%sDownloading... 100%%\n", p.indent)
	}
}

