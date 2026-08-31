// SPDX-License-Identifier: AGPL-3.0-or-later

package netcfg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// SimulatedKernel is a Runner that answers the way a Linux kernel answers,
// including by refusing.
//
// RecordingRunner says yes to everything. That is right for asserting which
// commands were generated and in what order, and it is blind to a whole class
// of defect: a double that always succeeds agrees with the code by
// construction, exactly like the sysctl double that formatted the shape its
// own parser wanted. Applying a plan twice looks perfect against a recorder
// and, on a real box, either stops at the first EEXIST or silently accumulates
// duplicate routing rules.
//
// So this one keeps state and models refusal. What it models, and how well it
// is known, measured on the target on 2026-08-30 (kernel 6.18.34,
// iproute2 6.15.0):
//
//   - "ip address add" and "ip route add" of something already there:
//     "RTNETLINK answers: File exists", exit 2. MEASURED for route.
//   - "ip rule add" of an identical selector at the SAME explicit priority:
//     "RTNETLINK answers: File exists", exit 2, and the rule list still holds
//     one rule afterwards. MEASURED. An earlier version of this file modelled
//     the kernel as accepting that silently and appending a duplicate. It does
//     not, and a double that is wrong in the safe direction still teaches a
//     false model to every reader and every future test.
//   - "ip rule add" with NO explicit priority: the kernel assigns one, so two
//     such adds become two rules. That is the shape where duplicates really do
//     accumulate and change which table wins, and it is modelled here because
//     it is the reason PlanNetwork must always emit a priority.
//   - "iw phy ... interface add" onto a taken name: "command failed: Invalid
//     exchange (-52)". MEASURED on an adjacent case, adding a name that
//     already exists as a netdev, and NOT on the case this package reaches.
//     The wording is deliberately one that IsAlreadyExists does not match, so
//     any test that survives a second apply is proving the precondition works
//     rather than proving a lucky string comparison.
//   - "ip -br link show dev <name>", the existence query that precondition
//     asks: absent exits 1 with `Device "<name>" does not exist.` on STDERR
//     and nothing on stdout; present exits 0 with the interface line on
//     stdout. MEASURED. The message is modelled on stderr rather than stdout
//     because a predicate reading a combined stream would take it as evidence
//     the device exists, which is the inverse of the truth.
type SimulatedKernel struct {
	mu sync.Mutex

	addrs     map[string]map[string]bool // device -> set of prefixes
	routes    map[string]map[string]bool // table -> set of prefixes
	rules     []simRule                  // ordered, duplicates permitted
	sysctl    map[string]string
	ifaces    map[string]bool // interfaces this kernel has
	unmanaged map[string]bool // devices released from NetworkManager
	iftype    map[string]string
	macs      map[string]string
	assoc     map[string]string // interface -> SSID it is joined to
	up        map[string]bool
	ifacePhy  map[string]string
	nft       int    // count of ruleset loads
	ruleset   string // the ruleset currently in force, "" when none
	cmds      []Command

	// Reads is consulted for any command this kernel does not model, so a
	// scenario's detection fixtures can still answer.
	Reads Runner

	// RefuseSetType, when non-empty, makes every "iw dev ... set type" fail
	// with this message.
	//
	// It was added on the expectation that a driver refusing to CREATE an
	// interface would probably refuse to change one. MEASURED on the target
	// on 2026-08-30: it does not. brcmfmac answers "Input/output error (-5)"
	// to "iw phy phy0 interface add" WHILE WLAN0 IS ASSOCIATED (and exit 0 to
	// the same add once wlan0 is not joined to anything), and exit 0 to
	// "iw dev wlan0 set type __ap", after which the interface reports type
	// AP. Creating an interface and changing one are different operations and
	// this driver treats them differently, which is the whole reason the
	// takeover path is worth having.
	//
	// The knob stays because another adapter may refuse, and because the test
	// it drives proves the rollback returns the user's WiFi. It no longer
	// models anything observed on this hardware.
	RefuseSetType string

	// InheritsParentMAC models a driver whose created interface takes the MAC
	// of the interface already on that radio, verbatim. MEASURED on the
	// target: "iw phy phy1 interface add captest type __ap" succeeded and
	// captest carried 02:00:5e:00:00:12, the same address as wlan1. Bringing
	// the second one up is then refused as a duplicate ADDRESS, which the
	// kernel words as "Name not unique on network".
	InheritsParentMAC bool

	// RefuseLinkUp, when non-empty, makes "ip link set dev X up" fail with
	// this message for an interface sharing a radio with one already up.
	// MEASURED: with a distinct MAC the same interface is refused with
	// "Device or resource busy", because the radio genuinely cannot hold an
	// access point beside an associated station.
	RefuseLinkUp string

	// RefuseIfaceAdd, when non-empty, makes every "iw phy ... interface add"
	// fail with this message and create nothing, modelling a driver that
	// refuses an interface combination its own capability table advertises.
	// MEASURED on the target: brcmfmac answers "command failed: Input/output
	// error (-5)", exit 251, for exactly that. See SimIwDriverRefuses.
	RefuseIfaceAdd string
}

// SimIwDriverRefuses is what the Pi 5's built-in radio answered on 2026-08-30
// when asked to add an access point interface beside its associated station,
// a combination its own "iw list" advertises.
const SimIwDriverRefuses = "command failed: Input/output error (-5)"

type simRule struct {
	priority string
	selector string
}

// NewSimulatedKernel returns an empty machine: no addresses, no routes, no
// rules, and only the interfaces named.
func NewSimulatedKernel(existingIfaces ...string) *SimulatedKernel {
	k := &SimulatedKernel{
		addrs:     map[string]map[string]bool{},
		routes:    map[string]map[string]bool{},
		sysctl:    map[string]string{},
		ifaces:    map[string]bool{},
		unmanaged: map[string]bool{},
		iftype:    map[string]string{},
		macs:      map[string]string{},
		assoc:     map[string]string{},
		up:        map[string]bool{},
		ifacePhy:  map[string]string{},
	}
	for _, n := range existingIfaces {
		k.ifaces[n] = true
		// A wireless interface starts as a station, which is what makes
		// "set type managed" on teardown return to the state it began in.
		k.iftype[n] = "managed"
		// And an interface the machine already has is up. This matters:
		// whether a second interface on the same radio can come up depends on
		// whether the first one is up, and a model where nothing is up cannot
		// express the refusal that was measured.
		k.up[n] = true
	}
	return k
}

// Preload puts state on the machine before anything is applied, which is how a
// test models an address, route or rule that was there first and that this
// package must not adopt.
func (k *SimulatedKernel) Preload(kind, a, b string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	switch kind {
	case "addr":
		k.ensureAddr(a)[b] = true
	case "route":
		k.ensureRoute(a)[b] = true
	case "rule":
		k.rules = append(k.rules, simRule{priority: a, selector: b})
	case "sysctl":
		k.sysctl[a] = b
	case "iface":
		k.ifaces[a] = true
		k.iftype[a] = "managed"
		k.up[a] = true
	case "mac":
		k.macs[a] = b
	case "assoc":
		k.assoc[a] = b
	case "ifacephy":
		k.ifacePhy[a] = b
	default:
		panic("netcfg: SimulatedKernel.Preload: unknown kind " + kind)
	}
}

func (k *SimulatedKernel) ensureAddr(dev string) map[string]bool {
	if k.addrs[dev] == nil {
		k.addrs[dev] = map[string]bool{}
	}
	return k.addrs[dev]
}

func (k *SimulatedKernel) ensureRoute(table string) map[string]bool {
	if k.routes[table] == nil {
		k.routes[table] = map[string]bool{}
	}
	return k.routes[table]
}

// Snapshot renders the whole machine as sorted lines, so a test can assert
// that teardown returned it to exactly the state it started in.
func (k *SimulatedKernel) Snapshot() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for dev, set := range k.addrs {
		for p := range set {
			out = append(out, "addr "+dev+" "+p)
		}
	}
	for table, set := range k.routes {
		for p := range set {
			out = append(out, "route "+table+" "+p)
		}
	}
	for _, r := range k.rules {
		out = append(out, "rule "+r.priority+" "+r.selector)
	}
	for kn, v := range k.sysctl {
		out = append(out, "sysctl "+kn+"="+v)
	}
	for n := range k.ifaces {
		out = append(out, "iface "+n)
	}
	for n, released := range k.unmanaged {
		if released {
			out = append(out, "unmanaged "+n)
		}
	}
	for n, t := range k.iftype {
		out = append(out, "iftype "+n+"="+t)
	}
	sort.Strings(out)
	return out
}

// Rules returns the rules currently installed, in order, so a test can count
// duplicates rather than infer their absence.
func (k *SimulatedKernel) Rules() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make([]string, 0, len(k.rules))
	for _, r := range k.rules {
		out = append(out, r.priority+": "+r.selector)
	}
	return out
}

// Commands returns every command this kernel was asked to run, in order.
func (k *SimulatedKernel) Commands() []Command {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make([]Command, len(k.cmds))
	copy(out, k.cmds)
	return out
}

// Lines renders the commands one per line.
func (k *SimulatedKernel) Lines() []string {
	cmds := k.Commands()
	out := make([]string, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, CommandLine(c))
	}
	return out
}

// The messages iproute2 and iw print. IsAlreadyExists matches the first pair.
const (
	simExists  = "RTNETLINK answers: File exists"
	simNoEntry = "RTNETLINK answers: No such file or directory"
	simNoDev   = "Cannot find device"
	// Measured on the target for an adjacent case; see the type comment.
	// Deliberately NOT a string IsAlreadyExists matches.
	simIwExists = "command failed: Invalid exchange (-52)"
	simIwNoDev  = "command failed: No such device (-19)"
)

func simFail(msg string) (Result, error) {
	return Result{Stderr: msg, ExitCode: 2}, errors.New(msg)
}

// Run implements Runner.
func (k *SimulatedKernel) Run(ctx context.Context, c Command) (Result, error) {
	if err := ValidateCommand(c); err != nil {
		return Result{}, err
	}
	k.mu.Lock()
	k.cmds = append(k.cmds, c)
	k.mu.Unlock()

	switch c.Path {
	case BinNft:
		k.mu.Lock()
		defer k.mu.Unlock()
		if len(c.Args) >= 2 && c.Args[0] == "list" {
			if k.ruleset == "" {
				msg := "Error: No such file or directory"
				return Result{Stderr: msg, ExitCode: 1}, errors.New(msg)
			}
			return Result{Stdout: k.ruleset}, nil
		}
		k.nft++
		// A load replaces whatever was there, which is what the
		// create-then-delete idiom in the generated ruleset does.
		k.ruleset = c.Stdin
		if strings.Contains(c.Stdin, "firewall teardown") {
			k.ruleset = ""
		}
		return Result{}, nil
	case BinSysctl:
		return k.runSysctl(ctx, c)
	case BinNmcli:
		return k.runNmcli(c)
	case BinIw:
		return k.runIw(ctx, c)
	case BinIP:
		return k.runIP(ctx, c)
	}
	return Result{}, fmt.Errorf("netcfg: SimulatedKernel does not model %q", c.Path)
}

func (k *SimulatedKernel) runSysctl(ctx context.Context, c Command) (Result, error) {
	// Writes always succeed and are idempotent, which is exactly why a second
	// apply must not re-derive their inverse from the machine.
	if len(c.Args) >= 2 && c.Args[0] == "-w" {
		key, val, ok := strings.Cut(c.Args[1], "=")
		if !ok {
			return simFail("sysctl: malformed setting")
		}
		k.mu.Lock()
		k.sysctl[key] = val
		k.mu.Unlock()
		return Result{}, nil
	}
	// A read. Answer from state where known, otherwise defer.
	var b strings.Builder
	k.mu.Lock()
	for _, a := range c.Args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if v, ok := k.sysctl[a]; ok {
			b.WriteString(a + " = " + v + "\n")
		}
	}
	k.mu.Unlock()
	if b.Len() > 0 {
		return Result{Stdout: b.String()}, nil
	}
	if k.Reads != nil {
		return k.Reads.Run(ctx, c)
	}
	return Result{}, nil
}

// runNmcli models the only two things this package asks of NetworkManager: a
// read-only device listing, and releasing one device.
func (k *SimulatedKernel) runNmcli(c Command) (Result, error) {
	a := c.Args
	k.mu.Lock()
	defer k.mu.Unlock()
	switch {
	case len(a) >= 5 && a[0] == "device" && a[1] == "set" && a[3] == "managed":
		name := a[2]
		if !k.ifaces[name] {
			return simFail("Error: Device '" + name + "' not found.")
		}
		k.unmanaged[name] = a[4] == "no"
		return Result{}, nil
	case len(a) >= 2 && a[len(a)-2] == "device" && a[len(a)-1] == "status":
		var b strings.Builder
		names := make([]string, 0, len(k.ifaces))
		for n := range k.ifaces {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			state := "connected"
			if k.unmanaged[n] {
				state = "unmanaged"
			}
			fmt.Fprintf(&b, "%s:%s\n", n, state)
		}
		return Result{Stdout: b.String()}, nil
	}
	return Result{}, nil
}

func (k *SimulatedKernel) runIw(ctx context.Context, c Command) (Result, error) {
	a := c.Args
	k.mu.Lock()
	switch {
	case len(a) == 3 && a[0] == "dev" && a[2] == "link":
		name := a[1]
		if !k.ifaces[name] {
			k.mu.Unlock()
			return simFail(simIwNoDev)
		}
		ssid := k.assoc[name]
		k.mu.Unlock()
		if ssid == "" {
			// MEASURED wording, including the full stop.
			return Result{Stdout: "Not connected.\n"}, nil
		}
		return Result{Stdout: fmt.Sprintf("Connected to 12:34:56:78:9a:bc (on %s)\n\tSSID: %s\n\tfreq: 2457\n", name, ssid)}, nil
	case len(a) >= 5 && a[0] == "dev" && a[2] == "set" && a[3] == "type":
		name := a[1]
		if k.RefuseSetType != "" {
			k.mu.Unlock()
			return simFail(k.RefuseSetType)
		}
		if !k.ifaces[name] {
			k.mu.Unlock()
			return simFail(simIwNoDev)
		}
		k.iftype[name] = a[4]
		k.mu.Unlock()
		return Result{}, nil
	case len(a) >= 5 && a[0] == "phy" && a[2] == "interface" && a[3] == "add":
		name := a[4]
		if k.RefuseIfaceAdd != "" {
			// The driver refuses and creates nothing, whatever the radio's
			// combination table advertises.
			k.mu.Unlock()
			return simFail(k.RefuseIfaceAdd)
		}
		if k.ifaces[name] {
			k.mu.Unlock()
			return simFail(simIwExists)
		}
		k.ifaces[name] = true
		k.iftype[name] = a[6]
		k.ifacePhy[name] = a[1]
		if k.InheritsParentMAC {
			for other, phy := range k.ifacePhy {
				if other != name && phy == a[1] && k.macs[other] != "" {
					k.macs[name] = k.macs[other]
					break
				}
			}
		}
		k.mu.Unlock()
		return Result{}, nil
	case len(a) >= 3 && a[0] == "dev" && a[2] == "del":
		name := a[1]
		if !k.ifaces[name] {
			k.mu.Unlock()
			return simFail(simIwNoDev)
		}
		delete(k.ifaces, name)
		delete(k.iftype, name)
		delete(k.macs, name)
		delete(k.assoc, name)
		delete(k.up, name)
		delete(k.ifacePhy, name)
		// A device that does not exist has no managed/unmanaged state.
		// NetworkManager enumerates devices; it does not hold a setting for a
		// name that is gone, and it re-enumerates one that reappears from
		// scratch, which is exactly why it claims a created interface. Keeping
		// the flag here would have made teardown look like it left something
		// behind when the thing the flag was about no longer exists.
		delete(k.unmanaged, name)
		k.mu.Unlock()
		return Result{}, nil
	}
	k.mu.Unlock()
	// "iw dev" and "iw list" are reads; the scenario's captured output
	// answers them.
	if k.Reads != nil {
		return k.Reads.Run(ctx, c)
	}
	return Result{}, nil
}

func (k *SimulatedKernel) runIP(ctx context.Context, c Command) (Result, error) {
	a := c.Args
	family := ""
	for len(a) > 0 && strings.HasPrefix(a[0], "-") {
		if a[0] == "-6" {
			family = "v6:"
		}
		a = a[1:]
	}
	if len(a) == 0 {
		return Result{}, nil
	}
	// Read verbs are answered by the scenario's fixtures, not from this
	// kernel's state: this type models the answers to CHANGES, and detection
	// needs the real captured output. "ip rule show" is the exception, and it
	// has to be, because the precondition that stops duplicate rules is only
	// as good as the list it reads.
	isRead := len(a) < 2 || a[1] == "show" || a[1] == "list"

	switch a[0] {
	case "link":
		// "ip -br link show dev <name>" is the existence check the access
		// point's creation step asks before it acts. It is answered from this
		// kernel's own state, because that is the state the step is about.
		if len(a) >= 4 && isRead && a[2] == "dev" {
			return k.runLinkShowDev(a[3])
		}
		if isRead {
			break
		}
		if len(a) >= 5 && a[1] == "set" && a[2] == "dev" && a[len(a)-1] == "up" {
			return k.runLinkUp(a[3])
		}
		// Any other "ip link set" is idempotent on a real kernel too.
		return Result{}, nil
	case "address", "addr":
		if isRead {
			break
		}
		return k.runIPAddr(a)
	case "route":
		if isRead {
			break
		}
		return k.runIPRoute(family, a)
	case "rule":
		return k.runIPRule(a)
	}
	if k.Reads != nil {
		return k.Reads.Run(ctx, c)
	}
	return Result{}, nil
}

// runLinkUp models the two walls MEASURED on the target when bringing up an
// interface created beside an associated station on the same radio.
func (k *SimulatedKernel) runLinkUp(name string) (Result, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.ifaces[name] {
		return simFail(simNoDev)
	}
	// Wall one: the created interface inherited its parent's MAC, and the
	// kernel refuses the duplicate ADDRESS with wording that names the NAME.
	if mac := k.macs[name]; mac != "" {
		for other, m := range k.macs {
			if other != name && m == mac && k.up[other] {
				return simFail("RTNETLINK answers: Name not unique on network")
			}
		}
	}
	// Wall two: with a distinct MAC, the radio itself refuses.
	if k.RefuseLinkUp != "" && k.ifacePhy[name] != "" {
		for other, phy := range k.ifacePhy {
			if other != name && phy == k.ifacePhy[name] && k.up[other] {
				return simFail(k.RefuseLinkUp)
			}
		}
	}
	k.up[name] = true
	return Result{}, nil
}

// runLinkShowDev answers the existence query. iproute2 exits non-zero and
// prints nothing when the device is absent.
func (k *SimulatedKernel) runLinkShowDev(name string) (Result, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if !k.ifaces[name] {
		msg := "Device \"" + name + "\" does not exist."
		return Result{Stderr: msg, ExitCode: 1}, errors.New(msg)
	}
	return Result{Stdout: name + "  DOWN  <BROADCAST,MULTICAST>\n"}, nil
}

func (k *SimulatedKernel) runIPAddr(a []string) (Result, error) {
	if len(a) < 4 {
		return Result{}, nil
	}
	verb, prefix := a[1], a[2]
	dev := argAfter(a, "dev")
	k.mu.Lock()
	defer k.mu.Unlock()
	if dev == "" || !k.ifaces[dev] {
		return simFail(simNoDev)
	}
	switch verb {
	case "add":
		if k.ensureAddr(dev)[prefix] {
			return simFail(simExists)
		}
		k.ensureAddr(dev)[prefix] = true
	case "del":
		if !k.ensureAddr(dev)[prefix] {
			return simFail(simNoEntry)
		}
		delete(k.addrs[dev], prefix)
	}
	return Result{}, nil
}

func (k *SimulatedKernel) runIPRoute(family string, a []string) (Result, error) {
	if len(a) < 3 {
		return Result{}, nil
	}
	verb, prefix := a[1], family+a[2]
	table := argAfter(a, "table")
	if table == "" {
		table = "main"
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	switch verb {
	case "add":
		if k.ensureRoute(table)[prefix] {
			return simFail(simExists)
		}
		k.ensureRoute(table)[prefix] = true
	case "del":
		if !k.ensureRoute(table)[prefix] {
			return simFail(simNoEntry)
		}
		delete(k.routes[table], prefix)
	}
	return Result{}, nil
}

func (k *SimulatedKernel) runIPRule(a []string) (Result, error) {
	if len(a) < 2 {
		return Result{}, nil
	}
	verb := a[1]
	prio := argAfter(a, "priority")
	// Everything except the verb and the priority pair is the selector.
	var sel []string
	for i := 2; i < len(a); i++ {
		if a[i] == "priority" {
			i++
			continue
		}
		sel = append(sel, a[i])
	}
	selector := strings.Join(sel, " ")

	k.mu.Lock()
	defer k.mu.Unlock()
	switch verb {
	case "add":
		if prio == "" {
			// No priority given, so the kernel picks one. Two such adds get
			// two different priorities and become two rules: this is the only
			// way a duplicate arises, and it is why every rule this package
			// generates carries an explicit priority.
			prio = k.nextFreePriority()
			k.rules = append(k.rules, simRule{priority: prio, selector: selector})
			return Result{}, nil
		}
		for _, r := range k.rules {
			if r.priority == prio && r.selector == selector {
				// MEASURED: the kernel refuses an identical rule at the same
				// explicit priority rather than duplicating it.
				return simFail(simExists)
			}
		}
		k.rules = append(k.rules, simRule{priority: prio, selector: selector})
		return Result{}, nil
	case "del":
		for i, r := range k.rules {
			if r.priority == prio && r.selector == selector {
				k.rules = append(k.rules[:i], k.rules[i+1:]...)
				return Result{}, nil
			}
		}
		return simFail(simNoEntry)
	case "show", "list":
		var b strings.Builder
		for _, r := range k.rules {
			fmt.Fprintf(&b, "%s:\t%s\n", r.priority, r.selector)
		}
		return Result{Stdout: b.String()}, nil
	}
	return Result{}, nil
}

// nextFreePriority mimics the kernel picking a priority for a rule that was
// added without one: the highest value below the lowest rule already present.
func (k *SimulatedKernel) nextFreePriority() string {
	p := 32765
	for {
		taken := false
		for _, r := range k.rules {
			if r.priority == strconv.Itoa(p) {
				taken = true
				break
			}
		}
		if !taken {
			return strconv.Itoa(p)
		}
		p--
	}
}

// argAfter returns the argument following the given keyword, or "".
func argAfter(args []string, key string) string {
	for i, a := range args {
		if a == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
