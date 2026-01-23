package syncmon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pushchain/push-validator-cli/internal/node"
)

type Options struct {
	LocalRPC     string
	RemoteRPC    string
	LogPath      string // Kept for compatibility but no longer used for state sync
	Window       int
	Compact      bool
	Out          io.Writer     // default os.Stdout
	Interval     time.Duration // refresh interval for progress updates
	Quiet        bool          // minimal, non-emoji, non-TTY style output
	Debug        bool          // extra diagnostic prints
	StuckTimeout time.Duration // timeout for detecting stalled sync
}

type pt struct {
	h int64
	t time.Time
}

const defaultStuckTimeout = 30 * time.Minute

var ErrSyncStuck = errors.New("sync stuck: no progress detected")

// RetryableError returns true if the error type allows automatic retry
func RetryableError(err error) bool {
	return errors.Is(err, ErrSyncStuck)
}

// Run monitors block sync progress via WebSocket header subscription.
func Run(ctx context.Context, opts Options) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Window <= 0 {
		opts.Window = 30
	}
	if opts.StuckTimeout <= 0 {
		opts.StuckTimeout = defaultStuckTimeout
	}
	lastProgress := newAtomicTime(time.Now())
	var rpcFailStart time.Time // zero = not failing
	const rpcFailTimeout = 30 * time.Second
	var lastTickHeight int64

	tty := isTTY()
	if opts.Quiet {
		tty = false
	}
	hideCursor(opts.Out, tty)
	defer showCursor(opts.Out, tty)

	// Block sync monitoring via WebSocket
	local := strings.TrimRight(opts.LocalRPC, "/")
	if local == "" {
		local = "http://127.0.0.1:26657"
	}
	hostport := hostPortFromURL(local)
	// Wait for RPC up to 60s
	if !waitTCP(hostport, 60*time.Second) {
		return fmt.Errorf("RPC not listening on %s", hostport)
	}
	cli := node.New(local)
	headers, err := cli.SubscribeHeaders(ctx)
	if err != nil {
		return fmt.Errorf("ws subscribe: %w", err)
	}

	// Remote (denominator) via WebSocket headers
	remote := strings.TrimRight(opts.RemoteRPC, "/")
	if remote == "" {
		remote = local
	}
	remoteCli := node.New(remote)
	remoteHeaders, remoteWSErr := remoteCli.SubscribeHeaders(ctx)

	buf := make([]pt, 0, opts.Window)
	var lastRemote int64
	var baseH int64
	var baseRemote int64
	var barPrinted bool
	var firstBarTime time.Time
	var holdStarted bool
	var lastPeers int
	var lastLatency int64
	var lastMetricsAt time.Time
	// minimum time to show the bar even if already synced
	const minShow = 15 * time.Second
	// Print initial line to claim space
	if tty {
		fmt.Fprint(opts.Out, "\r")
	}
	iv := opts.Interval
	if iv <= 0 {
		iv = 1 * time.Second
	}
	tick := time.NewTicker(iv)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case rhd, ok := <-remoteHeaders:
			if remoteWSErr == nil && ok {
				lastProgress.Update()
				lastRemote = rhd.Height
				var cur int64
				if len(buf) > 0 {
					cur = buf[len(buf)-1].h
				}
				if cur == 0 {
					ctx2, cancel2 := context.WithTimeout(context.Background(), 800*time.Millisecond)
					st, err := cli.Status(ctx2)
					cancel2()
					if err == nil {
						cur = st.Height
					}
				}
				if cur > 0 && baseH != 0 {
					percent := 0.0
					if lastRemote > 0 {
						percent = float64(cur) / float64(lastRemote) * 100
					}
					percent = floor2(percent)
					if cur < lastRemote && percent >= 100 {
						percent = 99.99
					}
					line := renderProgressWithQuiet(percent, cur, lastRemote, opts.Quiet)
					rate, eta := progressRateAndETA(buf, cur, lastRemote)
					lineWithETA := line
					if eta != "" {
						lineWithETA += eta
					}
					if tty {
						extra := ""
						if lastPeers > 0 {
							extra += fmt.Sprintf("  peers: %d", lastPeers)
						}
						if lastLatency > 0 {
							extra += fmt.Sprintf("  rtt: %dms", lastLatency)
						}
						fmt.Fprintf(opts.Out, "\r\033[K  %s%s", lineWithETA, extra)
					} else {
						if opts.Quiet {
							fmt.Fprintf(opts.Out, "  height=%d/%d rate=%.2f%s peers=%d rtt=%dms\n", cur, lastRemote, rate, eta, lastPeers, lastLatency)
						} else {
							fmt.Fprintf(opts.Out, "  %s height=%d/%d rate=%.2f blk/s%s peers=%d rtt=%dms\n", time.Now().Format(time.Kitchen), cur, lastRemote, rate, eta, lastPeers, lastLatency)
						}
					}
					if !barPrinted {
						fmt.Fprintln(opts.Out, "")
						firstBarTime = time.Now()
						holdStarted = true
						barPrinted = true
					}
				}
			}
		case h, ok := <-headers:
			if !ok {
				// Channel closed — WS disconnected (timeout or node died).
				if isNodeAlive(opts.LocalRPC) {
					// Node is alive but WS timed out (normal during block sync).
					// Fall back to tick-based progress monitoring only.
					headers = nil // nil channel blocks forever in select
					break
				}
				if isSyncedQuick(opts.LocalRPC) {
					return nil
				}
				return ErrSyncStuck
			}
			lastProgress.Update()
			buf = append(buf, pt{h.Height, time.Now()})
			if len(buf) > opts.Window {
				buf = buf[1:]
			}
			// Render progress
			var cur = h.Height
			// Establish baseline once we know remote height and have at least one header
			if baseH == 0 && lastRemote > 0 && len(buf) > 0 {
				baseH = buf[0].h
				baseRemote = lastRemote
				if baseRemote <= baseH {
					baseRemote = baseH + 1000
				}
			}
			var percent float64
			if lastRemote > 0 {
				// Use baseline calculation only when there's meaningful progress to track
				if baseH > 0 && lastRemote > baseH && (lastRemote-baseH) > 100 {
					denom := float64(lastRemote - baseH)
					if denom > 0 {
						percent = float64(cur-baseH) / denom * 100
					}
				} else {
					// Direct calculation for already-synced or near-synced nodes
					percent = float64(cur) / float64(lastRemote) * 100
				}
			}
			// Avoid rounding up to 100.00 before actually matching remote
			percent = floor2(percent)
			if cur < lastRemote && percent >= 100 {
				percent = 99.99
			}
			// Compute moving rate from recent headers and derive ETA string.
			rate, eta := progressRateAndETA(buf, cur, lastRemote)
			// Periodically refresh peers and remote latency (every ~3s)
			if time.Since(lastMetricsAt) > 3*time.Second {
				lastMetricsAt = time.Now()
				ctxp, cancelp := context.WithTimeout(context.Background(), 800*time.Millisecond)
				if plist, err := cli.Peers(ctxp); err == nil {
					lastPeers = len(plist)
				}
				cancelp()
				t0 := time.Now()
				ctxl, cancell := context.WithTimeout(context.Background(), 800*time.Millisecond)
				_, _ = remoteCli.RemoteStatus(ctxl, remote)
				cancell()
				lastLatency = time.Since(t0).Milliseconds()
			}
			// Only render the bar once baseline exists
			if baseH == 0 {
				break
			}
			line := renderProgressWithQuiet(percent, cur, lastRemote, opts.Quiet)
			lineWithETA := line
			if eta != "" {
				lineWithETA += eta
			}
			if tty {
				extra := ""
				if lastPeers > 0 {
					extra += fmt.Sprintf("  peers: %d", lastPeers)
				}
				if lastLatency > 0 {
					extra += fmt.Sprintf("  rtt: %dms", lastLatency)
				}
				fmt.Fprintf(opts.Out, "\r\033[K  %s%s", lineWithETA, extra)
			} else {
				if opts.Quiet {
					fmt.Fprintf(opts.Out, "height=%d/%d rate=%.2f%s peers=%d rtt=%dms\n", cur, lastRemote, rate, eta, lastPeers, lastLatency)
				} else {
					fmt.Fprintf(opts.Out, "%s height=%d/%d rate=%.2f blk/s%s peers=%d rtt=%dms\n", time.Now().Format(time.Kitchen), cur, lastRemote, rate, eta, lastPeers, lastLatency)
				}
			}
			if !barPrinted {
				fmt.Fprintln(opts.Out, "")
				firstBarTime = time.Now()
				holdStarted = true
			}
			barPrinted = true
		case <-tick.C:
			if opts.StuckTimeout > 0 && lastProgress.Since() > opts.StuckTimeout {
				// Before declaring stuck, check if node RPC is still alive
				if !isNodeAlive(opts.LocalRPC) {
					if tty {
						fmt.Fprint(opts.Out, "\r\033[K")
					}
					return ErrSyncStuck
				}
				// Node is alive, reset progress timer and continue waiting
				lastProgress.Update()
			}
			// Completion check via local status (cheap)
			ctx2, cancel2 := context.WithTimeout(context.Background(), 800*time.Millisecond)
			st, err := cli.Status(ctx2)
			cancel2()
			if err != nil {
				// RPC failed — track wall-clock time of continuous failure
				if rpcFailStart.IsZero() {
					rpcFailStart = time.Now()
				}
				if time.Since(rpcFailStart) > rpcFailTimeout {
					if tty {
						fmt.Fprint(opts.Out, "\r\033[K")
					}
					return ErrSyncStuck
				}
				break
			}
			rpcFailStart = time.Time{} // reset on success
			if st.Height > lastTickHeight {
				lastProgress.Update()
				lastTickHeight = st.Height
			}
			// If we haven't printed any bar yet (e.g., already synced), render a final bar once
			if !barPrinted {
				cur := st.Height
				remoteH := lastRemote
				if remoteH == 0 { // quick remote probe
					remoteH = probeRemoteOnce(opts.RemoteRPC, cur)
				}
				if remoteH < cur {
					remoteH = cur
				}
				// If already synced but local height not yet reported, align to remote
				if !st.CatchingUp && cur < remoteH {
					cur = remoteH
				}
				// Avoid printing a misleading bar when cur is 0; wait for actual height
				if cur == 0 {
					break
				}
				var percent float64
				if remoteH > 0 {
					percent = float64(cur) / float64(remoteH) * 100
				}
				percent = floor2(percent)
				if cur < remoteH && percent >= 100 {
					percent = 99.99
				}
				line := renderProgressWithQuiet(percent, cur, remoteH, opts.Quiet)
				rate, eta := progressRateAndETA(buf, cur, remoteH)
				lineWithETA := line
				if eta != "" {
					lineWithETA += eta
				}
				if tty {
					extra := ""
					if lastPeers > 0 {
						extra += fmt.Sprintf("  peers: %d", lastPeers)
					}
					if lastLatency > 0 {
						extra += fmt.Sprintf("  rtt: %dms", lastLatency)
					}
					fmt.Fprintf(opts.Out, "\r\033[K  %s%s", lineWithETA, extra)
				} else {
					if opts.Quiet {
						fmt.Fprintf(opts.Out, "height=%d/%d rate=%.2f%s peers=%d rtt=%dms\n", cur, remoteH, rate, eta, lastPeers, lastLatency)
					} else {
						fmt.Fprintf(opts.Out, "%s height=%d/%d rate=%.2f blk/s%s peers=%d rtt=%dms\n", time.Now().Format(time.Kitchen), cur, remoteH, rate, eta, lastPeers, lastLatency)
					}
				}
				firstBarTime = time.Now()
				holdStarted = true
				barPrinted = true
				if baseH == 0 && cur > 0 {
					baseH = cur
					baseRemote = remoteH
				}
			}
			// Active sync: update progress bar on every tick using current RPC status
			if st.CatchingUp && barPrinted {
				cur := st.Height
				if cur > 0 {
					// Add current height to buffer for rate calculation
					buf = append(buf, pt{cur, time.Now()})
					if len(buf) > opts.Window {
						buf = buf[1:]
					}
					// Calculate progress using baseline logic
					remoteH := lastRemote
					if remoteH == 0 {
						remoteH = probeRemoteOnce(opts.RemoteRPC, cur)
					}
					if remoteH < cur {
						remoteH = cur
					}
					var percent float64
					if remoteH > 0 {
						// Use baseline calculation for meaningful progress tracking
						if baseH > 0 && remoteH > baseH && (remoteH-baseH) > 100 {
							denom := float64(remoteH - baseH)
							if denom > 0 {
								percent = float64(cur-baseH) / denom * 100
							}
						} else {
							// Direct calculation for near-synced nodes
							percent = float64(cur) / float64(remoteH) * 100
						}
					}
					percent = floor2(percent)
					if cur < remoteH && percent >= 100 {
						percent = 99.99
					}
					// Render progress bar with current stats
					line := renderProgressWithQuiet(percent, cur, remoteH, opts.Quiet)
					rate, eta := progressRateAndETA(buf, cur, remoteH)
					lineWithETA := line
					if eta != "" {
						lineWithETA += eta
					}
					if tty {
						extra := ""
						if lastPeers > 0 {
							extra += fmt.Sprintf("  peers: %d", lastPeers)
						}
						if lastLatency > 0 {
							extra += fmt.Sprintf("  rtt: %dms", lastLatency)
						}
						fmt.Fprintf(opts.Out, "\r\033[K  %s%s", lineWithETA, extra)
					} else {
						if opts.Quiet {
							fmt.Fprintf(opts.Out, "height=%d/%d rate=%.2f%s peers=%d rtt=%dms\n", cur, remoteH, rate, eta, lastPeers, lastLatency)
						} else {
							fmt.Fprintf(opts.Out, "%s height=%d/%d rate=%.2f blk/s%s peers=%d rtt=%dms\n", time.Now().Format(time.Kitchen), cur, remoteH, rate, eta, lastPeers, lastLatency)
						}
					}
				}
			}
			// While within minShow and already not catching_up, keep the bar live-updating
			if !st.CatchingUp && barPrinted && time.Since(firstBarTime) < minShow {
				cur := st.Height
				if cur == 0 && len(buf) > 0 {
					cur = buf[len(buf)-1].h
				}
				remoteH := lastRemote
				if remoteH == 0 {
					remoteH = probeRemoteOnce(opts.RemoteRPC, cur)
				}
				if remoteH < cur {
					remoteH = cur
				}
				percent := 0.0
				if remoteH > 0 {
					percent = float64(cur) / float64(remoteH) * 100
				}
				percent = floor2(percent)
				if cur < remoteH && percent >= 100 {
					percent = 99.99
				}
				line := renderProgressWithQuiet(percent, cur, remoteH, opts.Quiet)
				rate, eta := progressRateAndETA(buf, cur, remoteH)
				lineWithETA := line
				if eta != "" {
					lineWithETA += eta
				}
				if tty {
					extra := ""
					if lastPeers > 0 {
						extra += fmt.Sprintf("  peers: %d", lastPeers)
					}
					if lastLatency > 0 {
						extra += fmt.Sprintf("  rtt: %dms", lastLatency)
					}
					fmt.Fprintf(opts.Out, "\r\033[K  %s%s", lineWithETA, extra)
				} else {
					if opts.Quiet {
						fmt.Fprintf(opts.Out, "height=%d/%d rate=%.2f%s peers=%d rtt=%dms\n", cur, remoteH, rate, eta, lastPeers, lastLatency)
					} else {
						fmt.Fprintf(opts.Out, "%s height=%d/%d rate=%.2f blk/s%s peers=%d rtt=%dms\n", time.Now().Format(time.Kitchen), cur, remoteH, rate, eta, lastPeers, lastLatency)
					}
				}
				continue
			}
			// End condition: catching_up is false AND minShow window has passed
			if !st.CatchingUp && holdStarted && time.Since(firstBarTime) >= minShow {
				cur := st.Height
				remoteH := lastRemote
				if remoteH == 0 {
					remoteH = probeRemoteOnce(opts.RemoteRPC, cur)
				}
				if remoteH < cur {
					remoteH = cur
				}
				percent := 0.0
				if remoteH > 0 {
					percent = float64(cur) / float64(remoteH) * 100
				}
				percent = floor2(percent)
				if cur < remoteH && percent >= 100 {
					percent = 99.99
				}
				line := renderProgressWithQuiet(percent, cur, remoteH, opts.Quiet)
				rate, eta := progressRateAndETA(buf, cur, remoteH)
				lineWithETA := line
				if eta != "" {
					lineWithETA += eta
				}
				if tty {
					extra := ""
					if lastPeers > 0 {
						extra += fmt.Sprintf("  peers: %d", lastPeers)
					}
					if lastLatency > 0 {
						extra += fmt.Sprintf("  rtt: %dms", lastLatency)
					}
					fmt.Fprintf(opts.Out, "\r\033[K  %s%s\n", lineWithETA, extra)
				} else {
					if opts.Quiet {
						fmt.Fprintf(opts.Out, "height=%d/%d rate=%.2f%s peers=%d rtt=%dms\n", cur, remoteH, rate, eta, lastPeers, lastLatency)
					} else {
						fmt.Fprintf(opts.Out, "%s height=%d/%d rate=%.2f blk/s%s peers=%d rtt=%dms\n", time.Now().Format(time.Kitchen), cur, remoteH, rate, eta, lastPeers, lastLatency)
					}
				}
				return nil
			}
		}
	}
}

// RetryOptions extends Options with retry configuration
type RetryOptions struct {
	Options
	MaxRetries int          // Max retry attempts (default: 3)
	ResetFunc  func() error // Function to reset data before retry
}

// RunWithRetry runs sync monitoring with automatic retry on failure
func RunWithRetry(ctx context.Context, opts RetryOptions) error {
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 3
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	var lastErr error
	for attempt := 0; attempt <= opts.MaxRetries; attempt++ {
		if attempt > 0 {
			// Log retry attempt
			fmt.Fprintf(opts.Out, "\n  Sync retry %d/%d...\n", attempt, opts.MaxRetries)

			// Reset data before retry (clear conflicting state)
			if opts.ResetFunc != nil {
				if err := opts.ResetFunc(); err != nil {
					return fmt.Errorf("failed to reset for retry: %w", err)
				}
			}

			// Wait before retry (exponential backoff: 10s, 20s, 30s)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt*10) * time.Second):
			}
		}

		err := Run(ctx, opts.Options)
		if err == nil {
			return nil // Success
		}

		lastErr = err
		if !RetryableError(err) {
			return err // Non-retryable error
		}

		// Log the failure type
		if errors.Is(err, ErrSyncStuck) {
			fmt.Fprintf(opts.Out, "\n  Sync appears stuck\n")
		}
	}

	return fmt.Errorf("sync failed after %d retries: %w", opts.MaxRetries, lastErr)
}

// --- helpers ---

type atomicTime struct {
	value atomic.Int64
}

func newAtomicTime(t time.Time) *atomicTime {
	at := &atomicTime{}
	at.Store(t)
	return at
}

func (a *atomicTime) Store(t time.Time) {
	a.value.Store(t.UnixNano())
}

func (a *atomicTime) Update() {
	a.Store(time.Now())
}

func (a *atomicTime) Since() time.Duration {
	last := a.value.Load()
	if last == 0 {
		return 0
	}
	return time.Since(time.Unix(0, last))
}

func hostPortFromURL(s string) string {
	u, err := url.Parse(s)
	if err == nil && u.Host != "" {
		return u.Host
	}
	return "127.0.0.1:26657"
}

// isSyncedQuick checks local RPC catching_up with a tiny timeout.
func isSyncedQuick(local string) bool {
	local = strings.TrimRight(local, "/")
	if local == "" {
		local = "http://127.0.0.1:26657"
	}
	httpc := &http.Client{Timeout: 1200 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, local+"/status", nil)
	resp, err := httpc.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	var payload struct {
		Result struct {
			SyncInfo struct {
				CatchingUp bool `json:"catching_up"`
			} `json:"sync_info"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false
	}
	return !payload.Result.SyncInfo.CatchingUp
}

// isNodeAlive checks if the node's RPC is responding
func isNodeAlive(local string) bool {
	local = strings.TrimRight(local, "/")
	if local == "" {
		local = "http://127.0.0.1:26657"
	}
	httpc := &http.Client{Timeout: 3 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, local+"/health", nil)
	resp, err := httpc.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == 200
}

func waitTCP(hostport string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		conn, err := (&net.Dialer{Timeout: 750 * time.Millisecond}).Dial("tcp", hostport)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(750 * time.Millisecond)
	}
	return false
}

// probeRemoteOnce fetches a single remote height with a small timeout.
func probeRemoteOnce(base string, fallback int64) int64 {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return fallback
	}
	httpc := &http.Client{Timeout: 1200 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/status", nil)
	resp, err := httpc.Do(req)
	if err != nil {
		return fallback
	}
	defer func() { _ = resp.Body.Close() }()
	var payload struct {
		Result struct {
			SyncInfo struct {
				Height string `json:"latest_block_height"`
			} `json:"sync_info"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fallback
	}
	h, _ := strconvParseInt(payload.Result.SyncInfo.Height)
	if h <= 0 {
		return fallback
	}
	return h
}

func movingRate(buf []struct {
	h int64
	t time.Time
}) float64 {
	n := len(buf)
	if n < 2 {
		return 0
	}
	dh := float64(buf[n-1].h - buf[0].h)
	dt := buf[n-1].t.Sub(buf[0].t).Seconds()
	if dt <= 0 {
		return 0
	}
	return dh / dt
}

func progressRateAndETA(buf []pt, cur, remote int64) (float64, string) {
	rate := movingRatePt(buf)
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		rate = 1.0
	}
	eta := ""
	if remote > cur && rate > 0 {
		rem := float64(remote-cur) / rate
		if rem < 0 {
			rem = 0
		}
		eta = fmt.Sprintf("  ETA %s", (time.Duration(rem * float64(time.Second))).Round(time.Second))
	} else if remote > 0 {
		eta = "  ETA 0s"
	}
	return rate, eta
}

func renderProgress(percent float64, cur, remote int64) string {
	width := 28
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("→ Syncing [%s] %.2f%%  %d/%d blocks", bar, percent, cur, remote)
}

func renderProgressWithQuiet(percent float64, cur, remote int64, quiet bool) string {
	if quiet {
		width := 28
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		filled := int(percent / 100 * float64(width))
		if filled < 0 {
			filled = 0
		}
		if filled > width {
			filled = width
		}
		bar := strings.Repeat("#", filled) + strings.Repeat("-", width-filled)
		return fmt.Sprintf("[%s] %.2f%%  %d/%d", bar, percent, cur, remote)
	}
	return renderProgress(percent, cur, remote)
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode()&os.ModeCharDevice) != 0 && os.Getenv("TERM") != ""
}

func hideCursor(w io.Writer, tty bool) {
	if tty {
		// Disable focus reporting (prevents ^[[I/^[[O sequences)
		fmt.Fprint(w, "\x1b[?1004l")
		// Hide cursor
		fmt.Fprint(w, "\x1b[?25l")
	}
}
func showCursor(w io.Writer, tty bool) {
	if tty {
		// Show cursor (don't re-enable focus reporting as it wasn't necessarily on before)
		fmt.Fprint(w, "\x1b[?25h")
	}
}

// local copy to avoid extra imports
func strconvParseInt(s string) (int64, error) {
	var n int64
	var sign int64 = 1
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	if s[0] == '-' {
		sign = -1
		s = s[1:]
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid")
		}
		n = n*10 + int64(c-'0')
	}
	return sign * n, nil
}

// floor2 returns v floored to two decimal places.
func floor2(v float64) float64 { return math.Floor(v*100.0) / 100.0 }

// convert helper to reuse movingRate signature
func movingRatePt(in []pt) float64 {
	tmp := make([]struct {
		h int64
		t time.Time
	}, len(in))
	for i := range in {
		tmp[i] = struct {
			h int64
			t time.Time
		}{h: in[i].h, t: in[i].t}
	}
	return movingRate(tmp)
}
