// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"caspianbyoc.org/caspian/internal/panel"
)

// Client is the panel's end of the socket. It implements panel.Privileged.
//
// It dials per call and does not hold a connection open. That is deliberate:
// internal/panel's unit is ordered after the privileged service with Wants= and
// never Requires=, because "a user who cannot reach the panel cannot fix
// anything", so the panel has to start and keep working while the privileged
// service is down, absent, or restarting. A client holding a connection would
// have to reconnect anyway, and a client that fails to construct would take the
// panel down with the thing it is meant to report on.
//
// A unix socket connect is cheap and the panel makes a handful of calls per
// page, so the cost of dialling per call is not worth a reconnect state machine.
type Client struct {
	path string

	// dialTimeout bounds the connect. A dial that hangs is the privileged
	// service being wedged, which the panel reports as unavailable.
	dialTimeout time.Duration
}

var _ panel.Privileged = (*Client)(nil)

// DefaultDialTimeout bounds one connect to the privileged service.
const DefaultDialTimeout = 3 * time.Second

// NewClient returns a client for the socket at path. It connects nothing: the
// first call does that, so a panel started before the privileged service is a
// panel that works.
func NewClient(path string) *Client {
	return &Client{path: path, dialTimeout: DefaultDialTimeout}
}

// Detect implements panel.Privileged.
func (c *Client) Detect(ctx context.Context) (panel.Detection, error) {
	resp, err := c.call(ctx, wireRequest{Action: panel.ActionDetect})
	if err != nil {
		return panel.Detection{}, err
	}
	if resp.Detect == nil {
		return panel.Detection{}, unexpectedReply(panel.ActionDetect)
	}
	return *resp.Detect, nil
}

// Status implements panel.Privileged.
func (c *Client) Status(ctx context.Context) (panel.SystemStatus, error) {
	resp, err := c.call(ctx, wireRequest{Action: panel.ActionStatus})
	if err != nil {
		return panel.SystemStatus{}, err
	}
	if resp.Status == nil {
		return panel.SystemStatus{}, unexpectedReply(panel.ActionStatus)
	}
	return *resp.Status, nil
}

// Start implements panel.Privileged.
func (c *Client) Start(ctx context.Context, req panel.StartRequest) error {
	_, err := c.call(ctx, wireRequest{Action: panel.ActionStart, Start: &req})
	return err
}

// Stop implements panel.Privileged.
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.call(ctx, wireRequest{Action: panel.ActionStop})
	return err
}

// Cut implements panel.Privileged.
func (c *Client) Cut(ctx context.Context) error {
	_, err := c.call(ctx, wireRequest{Action: panel.ActionCut})
	return err
}

// Recover implements panel.Privileged.
func (c *Client) Recover(ctx context.Context, req panel.StartRequest) error {
	_, err := c.call(ctx, wireRequest{Action: panel.ActionRecover, Start: &req})
	return err
}

// Restore implements panel.Privileged.
func (c *Client) Restore(ctx context.Context) error {
	_, err := c.call(ctx, wireRequest{Action: panel.ActionRestore})
	return err
}

// EngineLog implements panel.Privileged.
func (c *Client) EngineLog(ctx context.Context) (panel.EngineLog, error) {
	resp, err := c.call(ctx, wireRequest{Action: panel.ActionEngineLog})
	if err != nil {
		return panel.EngineLog{}, err
	}
	if resp.Log == nil {
		return panel.EngineLog{}, unexpectedReply(panel.ActionEngineLog)
	}
	return *resp.Log, nil
}

// call is the whole client protocol: dial, send one message, read one, close.
func (c *Client) call(ctx context.Context, req wireRequest) (wireResponse, error) {
	req.Version = protocolVersion
	if dl, ok := ctx.Deadline(); ok {
		req.DeadlineUnixNano = dl.UnixNano()
	}

	conn, err := dialEndpoint(ctx, c.path, c.dialTimeout)
	if err != nil {
		// The one place this package produces a panel.Fault on the client
		// side. internal/panel/priv.go: FaultUnavailable "is raised by this
		// package, not by the privileged side, for the obvious reason."
		return wireResponse{}, &panel.FaultError{Fault: panel.FaultUnavailable}
	}
	defer conn.Close()

	// A deadline in every case. internal/panel always gives one, and a caller
	// that does not must still not be able to block for ever on a privileged
	// service that has wedged: the socket read would never return, and the
	// goroutine holding it would be gone until the process restarted.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(MaxDeadline))
	}

	body, err := json.Marshal(req)
	if err != nil {
		return wireResponse{}, fmt.Errorf("privsvc: could not encode the request: %w", err)
	}
	if err := writeFrame(conn, body); err != nil {
		return wireResponse{}, c.transportError(ctx, err)
	}

	payload, err := readFrame(conn)
	if err != nil {
		return wireResponse{}, c.transportError(ctx, err)
	}

	var resp wireResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return wireResponse{}, errors.New("privsvc: the privileged service sent a reply this build cannot read")
	}
	if resp.Refusal != "" {
		return wireResponse{}, refusalError(resp.Refusal)
	}
	if resp.Fault != panel.FaultNone {
		// The fault word is turned back into the error type internal/panel
		// understands, so that panel.FaultOf on the other side of this call
		// reports exactly what the privileged side decided.
		return wireResponse{}, &panel.FaultError{Fault: resp.Fault}
	}
	return resp, nil
}

// transportError distinguishes "the caller gave up" from "the service went
// away", because the panel words them differently.
func (c *Client) transportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, errFrameTooLarge) {
		return err
	}
	return &panel.FaultError{Fault: panel.FaultUnavailable}
}

func unexpectedReply(a panel.Action) error {
	return fmt.Errorf("privsvc: the privileged service answered %q with no result", string(a))
}
