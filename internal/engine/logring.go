// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"sync"
	"time"

	applog "github.com/xtls/xray-core/app/log"
	clog "github.com/xtls/xray-core/common/log"
)

// DefaultLogCapacity is how many recent engine events an Engine keeps when
// New is used. It is a recent-events view for a panel, not an audit log, so
// the number is chosen to be readable on one screen after scrolling rather
// than to retain history.
const DefaultLogCapacity = 256

// LogEntry is one engine log line, already redacted.
type LogEntry struct {
	At   time.Time
	Text string
}

// LogRing is a bounded, oldest-first ring of engine log lines. Entries are
// redacted on the way in, never on the way out, so there is no path by which
// an unredacted string is ever held in memory here.
type LogRing struct {
	mu     sync.Mutex
	buf    []LogEntry
	next   int
	filled bool
	// dropped counts lines evicted by the ring. The panel needs to be able to
	// say "you are looking at the last 256 of 4,000", because a recent-events
	// view that silently hides the interesting failure is worse than no view.
	dropped uint64
	now     func() time.Time // overridable for tests
}

// NewLogRing returns a ring holding at most capacity entries. A capacity of
// zero or less is replaced with DefaultLogCapacity.
func NewLogRing(capacity int) *LogRing {
	if capacity <= 0 {
		capacity = DefaultLogCapacity
	}
	return &LogRing{buf: make([]LogEntry, capacity), now: time.Now}
}

// Add redacts text and appends it, evicting the oldest entry when full.
func (r *LogRing) Add(text string) {
	if r == nil {
		return
	}
	entry := LogEntry{At: r.now(), Text: Redact(text)}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.filled {
		r.dropped++
	}
	r.buf[r.next] = entry
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.filled = true
	}
}

// Entries returns a copy of the retained lines, oldest first.
func (r *LogRing) Entries() []LogEntry {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	n := r.next
	if r.filled {
		n = len(r.buf)
	}
	out := make([]LogEntry, 0, n)
	if r.filled {
		out = append(out, r.buf[r.next:]...)
	}
	out = append(out, r.buf[:r.next]...)
	return out
}

// Dropped reports how many lines have been evicted since the ring was created.
func (r *LogRing) Dropped() uint64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dropped
}

// Reset empties the ring. Used when an Engine starts, so the panel's
// recent-events view describes the run the user is looking at.
func (r *LogRing) Reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.buf {
		r.buf[i] = LogEntry{}
	}
	r.next = 0
	r.filled = false
	r.dropped = 0
}

// ---------------------------------------------------------------------------
// Wiring the engine's log output into the ring.
//
// The engine writes to stdout by default, on two separate routes, and both
// have to be closed or the appliance leaks engine text to a console nobody is
// reading and the panel shows nothing:
//
//  1. common/log/logger.go:182-184 has an init() that registers a stdout
//     handler as the process-global handler. That is in force from package
//     load until something replaces it, which covers everything this package
//     does before core.New.
//
//  2. During core.New, app/log.New (app/log/log.go:42) calls
//     log.RegisterHandler(g) and takes the global handler for itself. From
//     then on the destination is whatever app/log's own config says, and
//     infra/conf/log.go DefaultLogConfig sets ErrorLogType: LogType_Console,
//     which app/log/log_creator.go:44-46 resolves to
//     log.NewLogger(log.CreateStdoutLogWriter()).
//
// Closing route 1 is a RegisterHandler call. Closing route 2 by calling
// RegisterHandler again after core.New would work for later lines but would
// drop everything logged during feature construction, which is exactly the
// window where a bad config produces its most useful messages. So route 2 is
// closed at the source instead: this package replaces the LogType_Console
// handler creator (app/log/log_creator.go:21-30, an exported registration
// point) with one that writes into the ring. app/log then keeps doing its own
// job, including its severity filter, and its "console" simply is the ring.
//
// Both registrations are process-global. See the package doc: one Engine per
// process.
// ---------------------------------------------------------------------------

var (
	sinkMu      sync.Mutex
	sinkRing    *LogRing
	installOnce sync.Once
)

// ringSink is a clog.Handler that writes into whichever ring is currently
// active. It holds no reference of its own so that app/log may cache the
// handler it was given without pinning a stale ring.
type ringSink struct{}

func (ringSink) Handle(msg clog.Message) {
	sinkMu.Lock()
	r := sinkRing
	sinkMu.Unlock()
	if r == nil || msg == nil {
		return
	}
	r.Add(msg.String())
}

// captureEngineLogs points both global log routes at r. Safe to call repeatedly.
func captureEngineLogs(r *LogRing) {
	sinkMu.Lock()
	sinkRing = r
	sinkMu.Unlock()

	installOnce.Do(func() {
		// RegisterHandlerCreator only errors on a nil creator
		// (app/log/log_creator.go:22-24), so this cannot fail here.
		_ = applog.RegisterHandlerCreator(applog.LogType_Console,
			func(applog.LogType, applog.HandlerCreatorOptions) (clog.Handler, error) {
				return ringSink{}, nil
			})
	})

	// Route 1: take the global handler away from the stdout logger installed
	// by common/log's init. app/log will take it back during core.New, which
	// is fine, because by then its console creator is the one installed above.
	clog.RegisterHandler(ringSink{})
}

// forceLogToRing rewrites the app/log config the loader always produces so the
// engine's error log goes to our console creator and its access log goes
// nowhere.
//
// infra/conf/xray.go:512-519 prepends exactly one app/log config to
// config.App on every load, built from the user's "log" block or from
// DefaultLogConfig, so there is always one to find.
//
// Access logging is switched off rather than captured. app/log routes
// AccessMessage and DNSLog to the access logger (app/log/log.go:120-128), and
// those records are the destination host and the resolved name of every
// connection a client on the hotspot makes. Keeping them in a buffer that a
// web panel renders would build exactly the collection this product exists to
// avoid, so the access logger is set to LogType_None and dnsLog is cleared.
// The error log, which is the engine telling us about itself, is what the
// recent-events view is for.
//
// The user's loglevel is left alone: app/log's own severity filter at
// app/log/log.go:129-131 still applies, so "loglevel" in the pasted config
// keeps meaning what it means everywhere else.
func forceLogToRing(cfg *coreConfig) error {
	wanted := typedMessageName(&applog.Config{})

	for i, app := range cfg.App {
		if app.GetType() != wanted {
			continue
		}
		inst, err := app.GetInstance()
		if err != nil {
			return err
		}
		lc, ok := inst.(*applog.Config)
		if !ok {
			continue
		}
		lc.ErrorLogType = applog.LogType_Console
		lc.ErrorLogPath = ""
		lc.AccessLogType = applog.LogType_None
		lc.AccessLogPath = ""
		lc.EnableDnsLog = false

		msg, err := toTypedMessage(lc)
		if err != nil {
			return err
		}
		cfg.App[i] = msg
		return nil
	}

	// Not reachable with a config that came through the loader, but a caller
	// could hand us a hand-built one. Prepend rather than assume.
	lc := &applog.Config{
		ErrorLogType:  applog.LogType_Console,
		ErrorLogLevel: clog.Severity_Warning,
		AccessLogType: applog.LogType_None,
	}
	msg, err := toTypedMessage(lc)
	if err != nil {
		return err
	}
	cfg.App = append([]*typedMessage{msg}, cfg.App...)
	return nil
}
