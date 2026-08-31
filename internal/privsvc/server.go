// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
)

// SocketMode is the mode docs/LAYOUT.md fixes for /run/caspian/priv.sock.
const SocketMode fs.FileMode = 0o660

// ListenConfig is how to put the service on a socket.
type ListenConfig struct {
	// Path is the unix socket. docs/LAYOUT.md fixes /run/caspian/priv.sock.
	Path string

	// Group is the group to give the socket to, "caspian" on the appliance.
	// Empty means "leave the ownership alone", which is for a test or a
	// developer machine and never for the appliance: the mode below only means
	// something next to the right group.
	Group string

	// ServiceAccount is the account permitted to connect, besides root.
	// "caspian" on the appliance.
	ServiceAccount string

	// Logger receives the listener's own lines. nil discards them.
	Logger *slog.Logger

	// MaxConcurrent bounds connections being served at once. Zero means
	// DefaultMaxConcurrent. It is a bound and not a queue length: the panel
	// makes a handful of calls per page and anything beyond this is a fault or
	// an attempt to spend the box's memory.
	MaxConcurrent int

	// ReadTimeout bounds how long a connected peer may take to send its one
	// message. Zero means DefaultReadTimeout. A peer that connects and says
	// nothing must not hold a slot.
	ReadTimeout time.Duration

	// Now is the clock, overridable for tests.
	Now func() time.Time
}

// Defaults for ListenConfig.
const (
	DefaultMaxConcurrent = 16
	DefaultReadTimeout   = 10 * time.Second
)

// Listener serves one Service on a unix socket.
type Listener struct {
	svc     *Service
	ln      *net.UnixListener
	path    string
	log     *slog.Logger
	allowed Allowed
	now     func() time.Time
	sem     chan struct{}
	readTO  time.Duration

	closeOnce sync.Once
	wg        sync.WaitGroup
}

// ErrAlreadyRunning is returned when something is already listening on the
// socket.
var ErrAlreadyRunning = errors.New("privsvc: another copy of the privileged service is already listening on this socket")

// Listen binds the socket and returns a Listener that is not yet serving.
//
// The failures here are the ones somebody will actually hit, and each one says
// what it is rather than what it is not:
//
//   - something is already listening: the socket is dialled before anything is
//     removed, so a running service is never silently replaced;
//   - a leftover socket from a process that died: dialling it fails, and only
//     then is it removed;
//   - something at that path that is NOT a socket: refused and left alone,
//     because deleting a file this program did not create is not a repair.
func Listen(svc *Service, cfg ListenConfig) (*Listener, error) {
	if svc == nil {
		return nil, errors.New("privsvc: Listen needs a service")
	}
	if cfg.Path == "" {
		return nil, errors.New("privsvc: Listen needs a socket path; docs/LAYOUT.md fixes /run/caspian/priv.sock")
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	maxConc := cfg.MaxConcurrent
	if maxConc <= 0 {
		maxConc = DefaultMaxConcurrent
	}
	readTO := cfg.ReadTimeout
	if readTO <= 0 {
		readTO = DefaultReadTimeout
	}
	allowed, err := AllowedFor(cfg.ServiceAccount)
	if err != nil {
		// Not fatal: root can still drive the service, and a box with no
		// caspian account is a box where the panel was never installed. Saying
		// so beats refusing every connection with no explanation.
		log.Warn("the service account could not be resolved", "error", err.Error())
	}

	if err := prepareSocketPath(cfg.Path); err != nil {
		return nil, err
	}

	addr := &net.UnixAddr{Name: cfg.Path, Net: "unix"}
	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("privsvc: could not open the socket at %s: %w", cfg.Path, err)
	}
	// SetUnlinkOnClose is on by default for a listener that created the file;
	// it is stated so that a later reader does not have to know the default.
	ln.SetUnlinkOnClose(true)

	if err := secureSocket(cfg.Path, cfg.Group); err != nil {
		ln.Close()
		return nil, err
	}

	return &Listener{
		svc:     svc,
		ln:      ln,
		path:    cfg.Path,
		log:     log,
		allowed: allowed,
		now:     now,
		sem:     make(chan struct{}, maxConc),
		readTO:  readTO,
	}, nil
}

// prepareSocketPath makes the directory, and decides what to do about anything
// already at the path.
func prepareSocketPath(path string) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("privsvc: could not create the runtime directory %s: %w", dir, err)
		}
	}

	fi, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("privsvc: could not examine %s: %w", path, err)
	case fi.Mode()&fs.ModeSocket == 0:
		return fmt.Errorf("privsvc: %s exists and is not a socket, so this service will not remove it; "+
			"move it aside and start again", path)
	}

	// It is a socket. Dial it before removing it: a successful dial means
	// another copy of this service is running, and taking its socket away would
	// leave a root process serving nobody while this one served the panel.
	c, derr := net.DialTimeout("unix", path, 500*time.Millisecond)
	if derr == nil {
		c.Close()
		return fmt.Errorf("%w: %s", ErrAlreadyRunning, path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("privsvc: a leftover socket at %s could not be removed: %w", path, err)
	}
	return nil
}

// secureSocket applies the mode and ownership docs/LAYOUT.md fixes.
//
// The mode is set after the socket exists rather than through the umask,
// because the umask can only remove bits and this program does not get to
// choose what the umask was.
func secureSocket(path, group string) error {
	if err := os.Chmod(path, SocketMode); err != nil {
		return fmt.Errorf("privsvc: could not set the mode on %s: %w", path, err)
	}
	if group == "" {
		return nil
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return fmt.Errorf("privsvc: this machine has no %q group, so the socket cannot be given the ownership "+
			"docs/LAYOUT.md fixes: %w", group, err)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return fmt.Errorf("privsvc: the %q group has a group id this program cannot read: %w", group, err)
	}
	if err := os.Chown(path, 0, gid); err != nil {
		return fmt.Errorf("privsvc: could not give %s to root:%s, which is the ownership that makes mode %04o "+
			"a boundary rather than a decoration: %w", path, group, uint32(SocketMode), err)
	}
	return nil
}

// Addr is the socket this listener is on.
func (l *Listener) Addr() string { return l.path }

// Serve accepts connections until ctx is done or Close is called.
func (l *Listener) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		l.Close()
	}()

	for {
		conn, err := l.ln.AcceptUnix()
		if err != nil {
			l.wg.Wait()
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("privsvc: the socket stopped accepting connections: %w", err)
		}
		select {
		case l.sem <- struct{}{}:
		default:
			// Over the concurrency bound. Closing without a reply is right:
			// there is no Fault or Refusal for "come back later" and inventing
			// one would put a word on the wire that internal/panel cannot read.
			l.log.Warn("refused a connection: too many already being served")
			conn.Close()
			continue
		}
		l.wg.Add(1)
		go func() {
			defer func() {
				<-l.sem
				l.wg.Done()
			}()
			l.handle(ctx, conn)
		}()
	}
}

// Close stops accepting and removes the socket.
func (l *Listener) Close() error {
	var err error
	l.closeOnce.Do(func() { err = l.ln.Close() })
	return err
}

// handle serves one connection: one request, one response, then close.
func (l *Listener) handle(ctx context.Context, conn *net.UnixConn) {
	defer conn.Close()

	// ---------------------------------------------------------------------
	// Who is calling, before anything is read.
	//
	// The credentials come from the kernel, recorded at connect time, not from
	// anything in the message. A peer that is not allowed gets no reply at all:
	// there is nothing to say to it, and a refusal message would only confirm
	// that something is listening.
	// ---------------------------------------------------------------------
	peer, err := peerCredential(conn)
	if err != nil {
		l.log.Warn("refused a connection: the account on the other end could not be established",
			"error", err.Error())
		return
	}
	if !l.allowed.Permits(peer) {
		l.log.Warn("refused a connection from an account that may not drive this service",
			"peer", peer.String(), "pid", peer.PID)
		return
	}

	_ = conn.SetReadDeadline(l.now().Add(l.readTO))

	payload, err := readFrame(conn)
	if err != nil {
		switch {
		case errors.Is(err, errFrameTooLarge):
			l.reply(conn, wireResponse{Refusal: RefusalTooLarge})
			l.log.Warn("refused a message larger than this service accepts", "peer", peer.String())
		case errors.Is(err, io.EOF):
			// A peer that connected and said nothing. Common enough during a
			// restart that it is not worth a warning.
		default:
			l.reply(conn, wireResponse{Refusal: RefusalBadFrame})
			l.log.Warn("refused a malformed message", "peer", peer.String())
		}
		return
	}

	req, refusal := decodeRequest(payload)
	if refusal != "" {
		// NOTHING PRIVILEGED HAS RUN AT THIS POINT. decodeRequest is pure: it
		// parses bytes and compares the verb against panel.Actions. The service
		// is not touched until the line below.
		l.reply(conn, wireResponse{Refusal: refusal})
		l.log.Warn("refused a request", "peer", peer.String(), "refusal", string(refusal))
		return
	}

	callCtx, cancel := context.WithDeadline(ctx, deadlineFrom(req.DeadlineUnixNano, l.now()))
	defer cancel()

	resp := l.dispatch(callCtx, req)
	// The write deadline is the call's, so a peer that has gone away cannot
	// wedge the goroutine holding a concurrency slot.
	_ = conn.SetWriteDeadline(l.now().Add(l.readTO))
	l.reply(conn, resp)
}

// dispatch runs one named action. It is the only place in this file that
// reaches the service.
func (l *Listener) dispatch(ctx context.Context, req wireRequest) wireResponse {
	switch req.Action {
	case panel.ActionDetect:
		d, err := l.svc.Detect(ctx)
		if err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{Detect: &d}

	case panel.ActionStatus:
		st, err := l.svc.Status(ctx)
		if err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{Status: &st}

	case panel.ActionStart:
		if err := l.svc.Start(ctx, *req.Start); err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{}

	case panel.ActionRecover:
		if err := l.svc.Recover(ctx, *req.Start); err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{}

	case panel.ActionStop:
		if err := l.svc.Stop(ctx); err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{}

	case panel.ActionCut:
		if err := l.svc.Cut(ctx); err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{}

	case panel.ActionRestore:
		if err := l.svc.Restore(ctx); err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{}

	case panel.ActionEngineLog:
		lg, err := l.svc.EngineLog(ctx)
		if err != nil {
			return l.faultResponse(req.Action, err)
		}
		return wireResponse{Log: &lg}
	}
	// Unreachable: decodeRequest refused anything not in panel.Actions. It is
	// here so that an action added to internal/panel without a case above is a
	// refusal rather than a silent success.
	return wireResponse{Refusal: RefusalUnknownAction}
}

// faultResponse reduces an error to the one word that crosses the socket, and
// logs the rest on this side.
//
// This is the choke point the whole redaction argument rests on. Errors here can
// carry the engine's own text, which embeds the user's private key, seed, short
// id and UUID; a daemon's stderr; or a value the caller sent. None of it goes
// back, because the response carries a panel.Fault and there is no field on
// wireResponse for anything else.
func (l *Listener) faultResponse(action panel.Action, err error) wireResponse {
	f := faultOf(err)
	l.log.Warn("action failed", "action", string(action), "fault", string(f))
	return wireResponse{Fault: f}
}

func (l *Listener) reply(conn *net.UnixConn, resp wireResponse) {
	b, err := json.Marshal(resp)
	if err != nil {
		// Every field of wireResponse is a plain type from internal/panel, so
		// this cannot happen from any value produced above. It is handled
		// rather than ignored because a silent write of nothing looks to the
		// client like a service that hung up.
		l.log.Error("could not encode a reply")
		return
	}
	if err := writeFrame(conn, b); err != nil {
		l.log.Warn("could not send a reply", "error", err.Error())
	}
}
