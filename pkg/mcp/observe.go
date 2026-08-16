package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The MCP surface is public and anonymous by policy — that is how new users
// find it — so the safety story is observation, not identity. Two pieces:
//
//  1. Every tool call gets a usage line (tool, duration, heap delta) in a
//     LOG FILE, not just stdout. The 2026-08-16 outage taught us why: the
//     process's stdout lived in a tmux pane, the box died of a memory
//     balloon, and the reboot that recovered it destroyed the only record
//     of what was running. The file survives.
//  2. A watchdog samples the heap; past a threshold it dumps every
//     IN-FLIGHT call with its arguments plus a pprof heap profile. The
//     fatal call never returns, so end-of-call accounting alone would
//     record everything except the one request that mattered.

// usageLog writes to both stderr and a persistent file. The file lives
// beside the binary's working directory under .petri-pilot/ unless
// PILOT_USAGE_LOG names another path.
var (
	usageOnce sync.Once
	usageOut  *log.Logger
)

func usageLogger() *log.Logger {
	usageOnce.Do(func() {
		path := os.Getenv("PILOT_USAGE_LOG")
		if path == "" {
			home, _ := os.UserHomeDir()
			path = filepath.Join(home, ".petri-pilot", "mcp-usage.log")
		}
		os.MkdirAll(filepath.Dir(path), 0o755)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("mcp: usage log unavailable (%v); falling back to stderr only", err)
			usageOut = log.Default()
			return
		}
		usageOut = log.New(f, "", log.LstdFlags|log.LUTC)
	})
	return usageOut
}

// callerKey carries the HTTP caller's identity signals into tool handlers.
// The surface is anonymous by policy, so "identity" here means the repeat-
// user signals an open endpoint legitimately has: the client IP (nginx's
// X-Forwarded-For), the user agent, and the MCP session id. IP+UA repeating
// across days is a returning user; the session id groups one sitting.
type ctxCallerKey int

const callerKey ctxCallerKey = 0

type caller struct {
	IP, UA, Session string
}

// WithCaller is the HTTPContextFunc for the streamable server: it stashes
// the request's caller signals where ObserveMiddleware can log them.
func WithCaller(ctx context.Context, r *http.Request) context.Context {
	ip := r.Header.Get("X-Forwarded-For")
	if i := strings.IndexByte(ip, ','); i > 0 {
		ip = ip[:i]
	}
	if ip == "" {
		ip = r.RemoteAddr
		if i := strings.LastIndexByte(ip, ':'); i > 0 {
			ip = ip[:i]
		}
	}
	return context.WithValue(ctx, callerKey, caller{
		IP:      strings.TrimSpace(ip),
		UA:      r.Header.Get("User-Agent"),
		Session: r.Header.Get("Mcp-Session-Id"),
	})
}

func callerFrom(ctx context.Context) caller {
	c, _ := ctx.Value(callerKey).(caller)
	return c
}

// inflight tracks calls currently executing, so the watchdog can name what
// was running when memory climbed — including the call that never returns.
type inflightCall struct {
	Tool    string
	Started time.Time
	Args    string // truncated JSON of the request arguments
	Caller  caller
}

var (
	inflightMu sync.Mutex
	inflightID uint64
	inflight   = map[uint64]inflightCall{}
)

func truncArgs(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "<unmarshalable>"
	}
	const cap = 8 << 10
	if len(b) > cap {
		return string(b[:cap]) + fmt.Sprintf("…(%d bytes total)", len(b))
	}
	return string(b)
}

func heapMB() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc >> 20
}

// ObserveMiddleware logs one usage line per tool call and keeps the
// in-flight table current. Heap figures are process-wide, so concurrent
// calls can misattribute growth to each other — the line says what the
// process did while the call ran, which is exactly the right lead when
// one call is a balloon.
func ObserveMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		before := heapMB()
		start := time.Now()
		args := truncArgs(req.Params.Arguments)

		inflightMu.Lock()
		inflightID++
		id := inflightID
		who := callerFrom(ctx)
		inflight[id] = inflightCall{Tool: req.Params.Name, Started: start, Args: args, Caller: who}
		inflightMu.Unlock()
		defer func() {
			inflightMu.Lock()
			delete(inflight, id)
			inflightMu.Unlock()
		}()

		res, err := next(ctx, req)

		delta := int64(heapMB()) - int64(before)
		line := fmt.Sprintf("mcp: usage tool=%s ip=%s ua=%q sess=%s dur=%s heapMB=%d deltaMB=%+d",
			req.Params.Name, who.IP, who.UA, shortSession(who.Session), time.Since(start).Round(time.Millisecond), heapMB(), delta)
		if delta > argCaptureDeltaMB() {
			// A call that grew the heap this much is worth reproducing:
			// keep its arguments next to the number.
			line += " args=" + args
		}
		usageLogger().Print(line)
		return res, err
	}
}

func argCaptureDeltaMB() int64 {
	if v := os.Getenv("PILOT_MEM_ARGS_MB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return 128
}

// StartMemWatchdog samples the heap and, past thresholdMB (default 1024,
// override PILOT_MEM_DEBUG_MB), writes every in-flight call and a heap
// profile to the usage log directory. Rate-limited so a long balloon does
// not bury the disk. Call once from every entrypoint that serves MCP.
func StartMemWatchdog() {
	threshold := int64(1024)
	if v := os.Getenv("PILOT_MEM_DEBUG_MB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			threshold = n
		}
	}
	go func() {
		var lastDump time.Time
		for range time.Tick(15 * time.Second) {
			h := int64(heapMB())
			if h < threshold || time.Since(lastDump) < 10*time.Minute {
				continue
			}
			lastDump = time.Now()

			inflightMu.Lock()
			calls := make([]inflightCall, 0, len(inflight))
			for _, c := range inflight {
				calls = append(calls, c)
			}
			inflightMu.Unlock()

			lg := usageLogger()
			lg.Printf("mcp: MEMORY WATCHDOG heapMB=%d threshold=%d inflight=%d", h, threshold, len(calls))
			for _, c := range calls {
				lg.Printf("mcp: memdebug inflight tool=%s ip=%s running=%s args=%s", c.Tool, c.Caller.IP, time.Since(c.Started).Round(time.Second), c.Args)
			}

			home, _ := os.UserHomeDir()
			prof := filepath.Join(home, ".petri-pilot", fmt.Sprintf("heap-%s.pprof", time.Now().UTC().Format("20060102T150405Z")))
			if f, err := os.Create(prof); err == nil {
				runtime.GC()
				pprof.WriteHeapProfile(f)
				f.Close()
				lg.Printf("mcp: memdebug heap profile written to %s", prof)
			}
		}
	}()
}

// shortSession trims the session UUID to a correlatable stub — enough to
// group one sitting's calls, short enough to keep lines greppable.
func shortSession(s string) string {
	s = strings.TrimPrefix(s, "mcp-session-")
	if len(s) > 8 {
		return s[:8]
	}
	if s == "" {
		return "-"
	}
	return s
}
