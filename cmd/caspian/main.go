// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

// Command caspian is the whole appliance: one binary, and a subcommand chooses
// which half of it runs.
//
//	caspian serve --privileged   the root half: routes, firewall, access point, engine
//	caspian serve --panel        the unprivileged half: the web interface
//	caspian check                report what this box looks like, and change nothing
//	caspian version              what this binary is
//
// docs/LAYOUT.md, "Two processes, one binary", fixes the first two and says why
// they are separate: "The split exists so that a fault in the part that parses
// user input and serves HTTP is not a fault in the part that holds root."
//
// # Failing loudly
//
// The audience for this product cannot read a log, so every refusal this
// command can produce is written as a sentence that names what is wrong and,
// where there is one, what to do. "Cannot open the socket at
// /run/caspian/priv.sock: another copy of the privileged service is already
// listening" is the standard to meet. A stack trace is not.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Exit codes. They are few on purpose: a script can tell "it would not start"
// from "you typed it wrong", and nothing else is worth encoding.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "check":
		return runCheck(args[1:], stdout, stderr)
	case "version", "--version", "-version":
		writeVersion(stdout)
		return exitOK
	case "help", "--help", "-h":
		usage(stdout)
		return exitOK
	default:
		fmt.Fprintf(stderr, "caspian: %q is not a caspian command.\n\n", args[0])
		usage(stderr)
		return exitUsage
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `caspian - share a proxy connection over WiFi

Usage:
  caspian serve --privileged     run the root service: routes, firewall, access point, engine
  caspian serve --panel          run the web panel as the caspian user
  caspian check                  report what this box looks like; changes nothing
  caspian version                print the version of this binary

Options for "serve --panel":
  --listen HOST:PORT   serve on this address as well as the ones detected.
                       May be repeated. A wildcard or a public address is
                       refused. See "The panel has nowhere to listen on a
                       fresh box" in the notes accompanying this command.

"caspian check" takes no options. It runs read-only commands, prints what this
box looks like from two vantages, and changes nothing.

After the installer has run, everything a person does happens in the panel.
`)
}

// runServe dispatches to one of the two roles.
//
// Exactly one of the two flags is required, and asking for both is a usage
// error rather than a precedence rule. The two roles run as different accounts
// with different systemd hardening; a binary that quietly picked one when asked
// for both would be a binary that could be started with the wrong privileges by
// a typo.
func runServe(args []string, stdout, stderr io.Writer) int {
	var (
		privileged bool
		panelRole  bool
		listen     []string
	)
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--privileged":
			privileged = true
		case "--panel":
			panelRole = true
		case "--listen":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "caspian: --listen needs an address, for example --listen 192.168.4.31:8088")
				return exitUsage
			}
			i++
			listen = append(listen, args[i])
		default:
			fmt.Fprintf(stderr, "caspian: %q is not an option of \"caspian serve\".\n\n", a)
			usage(stderr)
			return exitUsage
		}
	}

	switch {
	case privileged && panelRole:
		fmt.Fprintln(stderr, "caspian: choose one of --privileged and --panel. They are two different services "+
			"running as two different accounts, and this binary will not run both.")
		return exitUsage
	case !privileged && !panelRole:
		fmt.Fprintln(stderr, "caspian: \"caspian serve\" needs --privileged or --panel.")
		return exitUsage
	case privileged && len(listen) > 0:
		fmt.Fprintln(stderr, "caspian: --listen belongs to \"serve --panel\". The privileged service listens on "+
			"the unix socket at "+socketPath+" and nowhere else.")
		return exitUsage
	}

	role := "caspian-panel"
	if privileged {
		role = "caspian"
	}
	ctx, stop := serviceContext(role)
	defer stop()

	log := newLogger(stderr)
	var err error
	if privileged {
		err = servePrivileged(ctx, log)
	} else {
		err = servePanel(ctx, log, listen)
	}
	if err != nil {
		// One sentence, on stderr, where systemd will record it. The error
		// values built in this package are already written as sentences; see
		// the note at the top of this file.
		fmt.Fprintf(stderr, "caspian: %v\n", err)
		return exitError
	}
	_ = stdout
	return exitOK
}

// newLogger writes the service's own lines to stderr, where systemd collects
// them.
//
// A text handler, not JSON: the person reading this is looking at
// "journalctl -u caspian" on a Raspberry Pi, and a line they can read beats a
// line a tool could parse. No colour: the project bans escape codes outright,
// and slog's text handler emits none.
func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
