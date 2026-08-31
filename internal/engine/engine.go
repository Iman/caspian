// SPDX-License-Identifier: AGPL-3.0-or-later

package engine

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/xtls/xray-core/core"

	// The engine registers its protocols, transports and config parsers from
	// init functions reached through this blank import. Without it a config
	// naming "vless" or "reality" fails to load with an unhelpful "unknown
	// protocol", because the JSON creator cache in infra/conf is populated by
	// the packages this pulls in.
	_ "github.com/xtls/xray-core/main/distro/all"

	// This is infra/conf/serial, the JSON config loader. It is NOT
	// common/serial, which is the protobuf TypedMessage package imported
	// below under an alias. The two have the same package name and different
	// jobs, and confusing them is the single easiest mistake to make in this
	// file.
	confserial "github.com/xtls/xray-core/infra/conf/serial"

	xserial "github.com/xtls/xray-core/common/serial"

	clog "github.com/xtls/xray-core/common/log"
	"google.golang.org/protobuf/proto"
)

// Aliases that keep the two serial packages apart at every use site.
type (
	coreConfig   = core.Config
	typedMessage = xserial.TypedMessage
)

func typedMessageName(m proto.Message) string { return xserial.GetMessageType(m) }

// toTypedMessage is xserial.ToTypedMessage with the marshal error kept.
// common/serial/typed_message.go:15 discards it, which would turn a
// marshalling bug into a silently empty app config.
func toTypedMessage(m proto.Message) (*typedMessage, error) {
	value, err := proto.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &typedMessage{Type: typedMessageName(m), Value: value}, nil
}

func init() {
	// common/log/logger.go:182-184 registers a stdout handler at package init.
	// Displace it as soon as this package is loaded, so that nothing the
	// engine says can reach a console before an Engine exists. Until New sets
	// a ring, ringSink drops what it is given, which for an appliance whose
	// only output is a web panel is the correct default.
	clog.RegisterHandler(ringSink{})
}

// Phase is the coarse lifecycle state of an Engine.
type Phase int

const (
	// PhaseStopped means no engine instance exists. This is the initial state
	// and the state after a successful Stop.
	PhaseStopped Phase = iota
	// PhaseStarting means a Start is in flight.
	PhaseStarting
	// PhaseRunning means an engine instance is constructed and started.
	PhaseRunning
	// PhaseFailed means the last Start did not complete. No instance is
	// running; State.Reason says why, redacted.
	PhaseFailed
)

func (p Phase) String() string {
	switch p {
	case PhaseStopped:
		return "stopped"
	case PhaseStarting:
		return "starting"
	case PhaseRunning:
		return "running"
	case PhaseFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// State is a snapshot of an Engine's lifecycle, safe to hand to the panel.
type State struct {
	Phase Phase
	// Reason is set only when Phase is PhaseFailed. It has already been
	// through Redact.
	Reason string
	// Since is when the Engine entered Phase.
	Since time.Time
}

// Error is an engine error whose message has already been through Redact.
//
// It deliberately does not implement Unwrap. The unredacted cause is dropped
// at the point of wrapping and cannot be recovered, so there is no way for a
// caller using errors.Unwrap, %+v or a logging helper to print the original
// text back out. That costs errors.Is and errors.As against engine errors,
// which this package does not need, and it buys the guarantee that the
// redaction cannot be undone downstream.
type Error struct {
	// Op is the operation that failed: "start", "stop" or "validate".
	Op string
	// Msg is the redacted message.
	Msg string
}

func (e *Error) Error() string { return e.Op + ": " + e.Msg }

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Op: op, Msg: Redact(err.Error())}
}

// Engine owns one xray-core instance.
//
// The zero value is not usable; call New.
type Engine struct {
	// opMu serialises Start and Stop against each other. It is held for the
	// whole of an operation, including the slow parts, and is never taken by
	// State, so a panel polling State is not blocked behind a start that is
	// waiting on the network.
	opMu sync.Mutex

	// mu guards everything below it.
	mu     sync.RWMutex
	phase  Phase
	reason string
	since  time.Time
	inst   *core.Instance

	// tun is what the running instance took from the machine and has to give
	// back: the tunnel device xray-core created inside core.New and the
	// descriptor it opened for it. Nothing in xray-core closes either one, so
	// this package does. See tundevice.go.
	tun *tunHold

	logs *LogRing
	now  func() time.Time

	// hookAfterConstruct runs after core.New has returned an instance and
	// before Start is called on it. It is nil in every build except the test
	// that needs the context to be cancelled inside that window, which is the
	// only way to prove the half-built instance is CLOSED rather than leaked:
	// the window is otherwise a race, and a test that tried to hit it by
	// timing would pass whether or not the Close was there.
	//
	// It is a field rather than a package variable so that tests setting it
	// cannot interfere with each other. Same pattern and same reason as
	// Store.hookAfterTempWrite in internal/state.
	hookAfterConstruct func()
}

// New returns a stopped Engine with a default-sized log ring.
func New() *Engine { return NewWithLogCapacity(DefaultLogCapacity) }

// NewWithLogCapacity returns a stopped Engine keeping at most capacity recent
// engine log lines.
func NewWithLogCapacity(capacity int) *Engine {
	e := &Engine{
		logs:  NewLogRing(capacity),
		now:   time.Now,
		phase: PhaseStopped,
	}
	e.since = e.now()
	// Claim the process log routes now rather than at Start, so that anything
	// the engine says during a Validate call also lands in the ring instead of
	// being dropped.
	captureEngineLogs(e.logs)
	return e
}

// State returns a snapshot. It never blocks on a Start or Stop in progress.
func (e *Engine) State() State {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return State{Phase: e.phase, Reason: e.reason, Since: e.since}
}

// Logs returns the retained engine log lines, oldest first, already redacted.
func (e *Engine) Logs() []LogEntry { return e.logs.Entries() }

// LogsDropped reports how many log lines the ring has evicted, so a caller can
// say that the view is truncated instead of implying it is complete.
func (e *Engine) LogsDropped() uint64 { return e.logs.Dropped() }

func (e *Engine) setState(p Phase, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.phase = p
	e.reason = reason
	e.since = e.now()
}

// Start loads configJSON and starts the engine.
//
// Start is idempotent: if the engine is already running, Start returns nil and
// does nothing, so two concurrent callers cannot leave two instances running
// and a panel that sends the same "on" twice does not restart the tunnel.
// Changing config is therefore Stop followed by Start, not Start alone;
// nothing here compares the new bytes to the running ones.
//
// ctx aborts the attempt at the two points where that is possible: before any
// work, and between constructing the instance and starting it. Neither
// core.New nor (*core.Instance).Start is interruptible, so a cancel that
// arrives inside one of them takes effect when it returns, and the partly
// built instance is closed before the error is returned.
//
// The returned error is always *Error, so it is already redacted.
func (e *Engine) Start(ctx context.Context, configJSON []byte) error {
	e.opMu.Lock()
	defer e.opMu.Unlock()

	if e.State().Phase == PhaseRunning {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return wrap("start", err)
	}

	e.setState(PhaseStarting, "")

	inst, hold, err := e.buildAndStart(ctx, configJSON)
	if err != nil {
		werr := wrap("start", err)
		e.setState(PhaseFailed, werr.(*Error).Msg)
		return werr
	}

	e.mu.Lock()
	e.inst = inst
	e.tun = hold
	e.mu.Unlock()
	e.setState(PhaseRunning, "")
	return nil
}

func (e *Engine) buildAndStart(ctx context.Context, configJSON []byte) (*core.Instance, *tunHold, error) {
	// The ring describes the run the panel is looking at, not the one before
	// it, so clear it before anything can log.
	e.logs.Reset()
	captureEngineLogs(e.logs)

	cfg, err := loadConfig(configJSON)
	if err != nil {
		return nil, nil, err
	}
	if err := forceLogToRing(cfg); err != nil {
		return nil, nil, err
	}

	// Recorded before core.New, because core.New is what creates the tunnel
	// device and opens its descriptor. The difference between this set and the
	// one taken afterwards is what this engine opened and nothing else did.
	beforeTun := tunDescriptors()

	// core.New with the background context rather than
	// core.NewWithContext(ctx, ...): the instance stores whatever context it
	// is given for its whole lifetime (core/xray.go:168 and :179) and hands it
	// to every feature, so a request-scoped ctx from a panel handler would
	// tear the tunnel down when the HTTP request ended.
	inst, err := core.New(cfg)
	if err != nil {
		// core.New adds the inbound handlers one at a time and returns on the
		// first that fails, so an earlier TUN inbound can have created a
		// device while there is no instance to close. This is the one leak
		// path the instance lifecycle cannot cover at all.
		e.releaseAfterFailedStart(captureTunHold(cfg, beforeTun))
		return nil, nil, err
	}
	hold := captureTunHold(cfg, beforeTun)

	if e.hookAfterConstruct != nil {
		e.hookAfterConstruct()
	}
	if err := ctx.Err(); err != nil {
		_ = inst.Close()
		e.releaseAfterFailedStart(hold)
		return nil, nil, err
	}
	if err := inst.Start(); err != nil {
		// core/xray.go:380 warns that after a failed Start the instance state
		// is unknown, so it is closed rather than kept or retried.
		_ = inst.Close()
		e.releaseAfterFailedStart(hold)
		return nil, nil, err
	}
	return inst, hold, nil
}

// releaseAfterFailedStart gives the tunnel device back after a start that did
// not complete.
//
// Closing the instance is not enough and on the core.New path there is not
// even an instance: see tundevice.go. The error is logged to the ring rather
// than returned, because the caller is already carrying the error that made
// the start fail and that is the one the user has to see. Same reasoning as
// internal/privsvc's rollbackLocked.
//
// This path matters more than it looks. The person who retries a bad
// configuration is exactly the person who would otherwise accumulate a device
// per attempt, and the second attempt would then fail for a different reason
// than the first.
func (e *Engine) releaseAfterFailedStart(hold *tunHold) {
	if err := hold.release(); err != nil {
		e.logs.Add("the tunnel device was not released after a start that failed: " + err.Error())
	}
}

// Stop closes the running instance. Stopping an engine that is not running is
// not an error and returns nil.
//
// After Stop the phase is PhaseStopped whether or not Close reported a
// problem, because the instance is discarded either way and a
// core.Instance cannot be restarted (core/xray.go:381-382). A non-nil return
// therefore means "it stopped, and closing these features complained", not
// "it is still running".
//
// Stop also removes the tunnel device, which closing the instance does not do.
// tundevice.go has the whole of why; the short version is that without it the
// next Start fails with "device or resource busy" and the appliance cannot be
// switched on again until the process restarts.
func (e *Engine) Stop() error {
	e.opMu.Lock()
	defer e.opMu.Unlock()

	e.mu.Lock()
	inst := e.inst
	hold := e.tun
	e.inst = nil
	e.tun = nil
	e.mu.Unlock()

	if inst == nil {
		// No instance, but the device is released anyway rather than on the
		// way past an early return. Today the two are always set and cleared
		// together, so this is a no-op that TestStopReleasesTheHoldEvenWithNoInstance
		// pins; it is written this way so that a later change which sets one
		// without the other cannot reintroduce the leak silently.
		rerr := hold.release()
		e.setState(PhaseStopped, "")
		return wrap("stop", rerr)
	}

	err := inst.Close()
	rerr := hold.release()

	// Closing the instance closes app/log's Instance too
	// (app/log/log.go:139-160), which nils its handlers and makes it swallow
	// everything. Take the global handler back so that anything the engine
	// says after shutdown is still captured rather than lost.
	captureEngineLogs(e.logs)

	e.setState(PhaseStopped, "")
	return wrap("stop", errors.Join(err, rerr))
}

// Validate loads configJSON and reports whether the engine accepts it, without
// starting anything.
//
// It does not call core.New, so no feature is constructed, no inbound handler
// is created and no socket is opened. The work is decode plus Build:
// infra/conf/serial/loader.go:72-85 decodes the JSON into a conf.Config and
// calls Build, which converts it to protobuf. Build reads from disk if the
// config names a certificate file or a geo data file; it does not open a
// listener and it does not dial.
//
// This is a schema check, not a completeness check. It is worth stating what
// it will not catch, because the panel's error message depends on it: the
// loader has no DisallowUnknownFields, so a misspelled key is accepted and
// dropped; and common/uuid/uuid.go:71-79 turns any 1-to-30-character string
// that is not a UUID into a different valid UUID by SHA-1, with no error, so a
// truncated id passes here and then authenticates as somebody else. Validating
// the pasted link's own fields is the parser's job, not this one's.
//
// The returned error is always *Error, so it is already redacted.
func Validate(configJSON []byte) error {
	_, err := loadConfig(configJSON)
	return wrap("validate", err)
}

func loadConfig(configJSON []byte) (*coreConfig, error) {
	return confserial.LoadJSONConfig(bytes.NewReader(configJSON))
}
