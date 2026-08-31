// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build linux

package netcfg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// systemRunner runs commands for real. It exists only on Linux; see
// exec_other.go for every other platform.
type systemRunner struct {
	// lookPath is exec.LookPath in production and is replaced in tests.
	lookPath func(string) (string, error)
}

// NewSystemRunner returns the runner that actually changes the machine.
//
// It is the only impure thing in this package, and it is deliberately small:
// validate the command against the allowlist, resolve the binary as a file,
// run it with an argument vector. There is no shell, so nothing that reaches
// an argument can become a command; the privileged side of the appliance is
// meant to accept named actions and never a command line, and this is where
// that stops being a convention.
func NewSystemRunner() Runner {
	return &systemRunner{lookPath: exec.LookPath}
}

// searchPath is where the tools are looked for. It is fixed rather than
// inherited so that a PATH from the environment cannot decide which binary
// called "ip" gets root.
var searchPath = []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin", "/usr/local/sbin", "/usr/local/bin"}

func (s *systemRunner) resolve(name string) (string, error) {
	for _, dir := range searchPath {
		p := dir + "/" + name
		if fi, err := lstatRegularExecutable(p); err == nil && fi {
			return p, nil
		}
	}
	if s.lookPath != nil {
		if p, err := s.lookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("netcfg: %q not found in %s", name, strings.Join(searchPath, ":"))
}

// Run implements Runner.
func (s *systemRunner) Run(ctx context.Context, c Command) (Result, error) {
	if err := ValidateCommand(c); err != nil {
		return Result{}, err
	}
	path, err := s.resolve(c.Path)
	if err != nil {
		return Result{}, err
	}

	cmd := exec.CommandContext(ctx, path, c.Args...)
	// An empty environment, not the inherited one. None of these tools needs a
	// variable from the caller, and an inherited environment is another way
	// for something outside this package to change what a privileged command
	// does.
	cmd.Env = []string{}
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		res.ExitCode = ee.ExitCode()
		return res, fmt.Errorf("netcfg: %s exited %d: %s", c.Path, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	if runErr != nil {
		return res, fmt.Errorf("netcfg: run %s: %w", c.Path, runErr)
	}
	return res, nil
}
