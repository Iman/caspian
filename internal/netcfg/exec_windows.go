// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows && (amd64 || arm64)

package netcfg

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

// windowsRunner carries out the Windows backend's pseudo-binaries in process.
//
// Nothing here execs anything. "iphlpapi" is the IP Helper API, "wfp" the
// Windows Filtering Platform, "wintun" the tunnel adapter driver. Each Command
// is still validated against the platform allowlist, still journalled by the
// shared Applier, and still answered as a Result whose stderr carries the
// wording the shared idempotence rules already recognise ("object exists",
// "not in table"), so a route that is already there or already gone is
// skipped and its inverse retracted exactly as on Linux.
//
// Everything privileged here needs an elevated token; cmd/caspian refuses to
// start the privileged role without one.
type windowsRunner struct {
	mu sync.Mutex
	// tuns holds the adapters this process created. A Wintun adapter is
	// removed when the handle that created it is closed, so the handle is
	// kept for the adapter's whole life and closed by "wintun delete".
	tuns map[string]*wintun.Adapter
}

// NewSystemRunner returns the runner that actually changes the machine.
func NewSystemRunner() Runner { return &windowsRunner{tuns: map[string]*wintun.Adapter{}} }

// SystemBackend is the backend for the machine this binary runs on.
func SystemBackend() Backend { return BackendFor(PlatformWindows) }

func (w *windowsRunner) Run(_ context.Context, c Command) (Result, error) {
	if err := ValidateCommandOn(PlatformWindows, c); err != nil {
		return Result{}, err
	}
	var (
		out string
		err error
	)
	switch c.Path {
	case BinIPHelper:
		out, err = w.ipHelper(c.Args)
	case BinWFP:
		err = w.wfp(c.Args, c.Stdin)
	case BinWintun:
		err = w.wintun(c.Args)
	default:
		err = fmt.Errorf("%w: %q", ErrDisallowedBinary, c.Path)
	}
	if err != nil {
		res := Result{Stderr: err.Error(), ExitCode: 1}
		return res, fmt.Errorf("netcfg: %s exited 1: %s", c.Path, err.Error())
	}
	return Result{Stdout: out}, nil
}

// classify turns a Windows error into the wording the shared markers read.
func classify(what string, err error) error {
	var e windows.Errno
	if errors.As(err, &e) {
		switch uint32(e) {
		case errorObjectAlreadyExists, fwpEAlreadyExists:
			return fmt.Errorf("%s: object exists", what)
		case errorNotFound, fwpEFilterNotFound, fwpESublayerNotFound, fwpEProviderNotFound:
			return fmt.Errorf("%s: not in table", what)
		}
	}
	return fmt.Errorf("%s: %w", what, err)
}

// ---------------------------------------------------------------------------
// iphlpapi
// ---------------------------------------------------------------------------

func (w *windowsRunner) ipHelper(args []string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("iphlpapi: no operation")
	}
	switch args[0] {
	case "adapters":
		inv, err := readInventory()
		if err != nil {
			return "", err
		}
		b, err := json.Marshal(inv)
		return string(b) + "\n", err
	case "route":
		return "", routeCommand(args[1:])
	case "addr":
		return "", addrCommand(args[1:])
	case "iface":
		return "", ifaceCommand(args[1:])
	}
	return "", fmt.Errorf("iphlpapi: unknown operation %q", args[0])
}

func luidForAlias(alias string) (netLUID, error) {
	p, err := windows.UTF16PtrFromString(alias)
	if err != nil {
		return 0, err
	}
	var luid netLUID
	if err := call(procConvertInterfaceAliasToLuid, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&luid))); err != nil {
		return 0, classify("adapter "+alias, err)
	}
	return luid, nil
}

func aliasForLUID(luid netLUID) string {
	var buf [ifMaxStringSize + 1]uint16
	if err := call(procConvertInterfaceLuidToAlias, uintptr(unsafe.Pointer(&luid)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:])
}

func readInventory() (WindowsInventory, error) {
	var inv WindowsInventory

	var ifTable *mibIfTable2
	if err := call(procGetIfTable2Ex, 0, uintptr(unsafe.Pointer(&ifTable))); err != nil {
		return inv, classify("GetIfTable2Ex", err)
	}
	defer procFreeMibTable.Call(uintptr(unsafe.Pointer(ifTable)))

	byIndex := map[uint32]*WindowsAdapter{}
	upByLUID := map[netLUID]bool{}
	for i := range ifTable.rows() {
		row := &ifTable.rows()[i]
		// NDIS layers a filter driver's own pseudo-adapter beside the real one
		// it is bound to (a WiFi radio carries four or five of these: the WFP
		// native and 802.3 MAC-layer LWFs, the QoS packet scheduler, the
		// Virtual and Native WiFi filter drivers). GetIfTable2Ex reports them
		// as ordinary IEEE 802.11 or Ethernet rows with their own index and no
		// address, so left unfiltered every one of them reads as another radio
		// or another uplink candidate. MEASURED on this machine: every such
		// row, and no real adapter, carries FilterInterface in this flag byte;
		// GetAdaptersAddresses already excludes them from its own list, which
		// is why they show no address either way.
		if row.InterfaceAndOperStatusFlags&mibIfFilterInterface != 0 {
			continue
		}
		a := WindowsAdapter{
			Alias: row.aliasString(),
			Index: int(row.InterfaceIndex),
			Up:    row.OperStatus == ifOperStatusUp,
		}
		switch row.Type {
		case ifTypeEthernet:
			a.Type = "ethernet"
		case ifTypeIEEE80211:
			a.Type = "wifi"
		case ifTypeLoopback:
			a.Type = "loopback"
		case ifTypeTunnel, ifTypePropVirtual:
			a.Type = "tunnel"
		default:
			a.Type = "other"
		}
		if strings.Contains(row.descriptionString(), "Wi-Fi Direct Virtual Adapter") {
			a.WiFiDirect = true
		}
		if a.Type != "loopback" {
			var ipRow mibIPInterfaceRow
			ipRow.Family = afInet
			ipRow.InterfaceLUID = row.InterfaceLUID
			if err := call(procGetIpInterfaceEntry, uintptr(unsafe.Pointer(&ipRow))); err == nil {
				a.Forwarding = ipRow.ForwardingEnabled
			}
		}
		upByLUID[row.InterfaceLUID] = a.Up
		inv.Adapters = append(inv.Adapters, a)
		byIndex[row.InterfaceIndex] = &inv.Adapters[len(inv.Adapters)-1]
	}

	// Addresses, through GetAdaptersAddresses, matched to the table by index.
	const flags = windows.GAA_FLAG_SKIP_ANYCAST | windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER
	size := uint32(15 * 1024)
	var buf []byte
	for attempt := 0; attempt < 4; attempt++ {
		buf = make([]byte, size)
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_BUFFER_OVERFLOW) {
			return inv, classify("GetAdaptersAddresses", err)
		}
	}
	for aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); aa != nil; aa = aa.Next {
		target := byIndex[aa.IfIndex]
		if target == nil && aa.Ipv6IfIndex != 0 {
			target = byIndex[aa.Ipv6IfIndex]
		}
		if target == nil {
			continue
		}
		for ua := aa.FirstUnicastAddress; ua != nil; ua = ua.Next {
			ip := ua.Address.IP()
			if ip == nil {
				continue
			}
			addr, ok := netip.AddrFromSlice(ip)
			if !ok {
				continue
			}
			target.Prefixes = append(target.Prefixes, netip.PrefixFrom(addr.Unmap(), int(ua.OnLinkPrefixLength)).String())
		}
	}

	// Default routes.
	var fwd *mibIPforwardTable2
	if err := call(procGetIpForwardTable2, uintptr(afUnspec), uintptr(unsafe.Pointer(&fwd))); err != nil {
		return inv, classify("GetIpForwardTable2", err)
	}
	defer procFreeMibTable.Call(uintptr(unsafe.Pointer(fwd)))
	for _, r := range fwd.rows() {
		if r.DestinationPrefix.PrefixLength != 0 {
			continue
		}
		d := WindowsDefaultRoute{Alias: aliasForLUID(r.InterfaceLUID), Metric: int(r.Metric), Up: upByLUID[r.InterfaceLUID]}
		switch r.DestinationPrefix.RawPrefix.Family {
		case afInet6:
			d.Family = 6
		default:
			d.Family = 4
		}
		if gw := r.NextHop.addr(); gw.IsValid() && !gw.IsUnspecified() {
			d.Gateway = gw.String()
		}
		inv.Defaults = append(inv.Defaults, d)
	}
	return inv, nil
}

// routeCommand: add|delete <prefix> dev <alias> [via <gateway>] metric <n>
func routeCommand(args []string) error {
	if len(args) < 4 || args[2] != "dev" {
		return errors.New("iphlpapi route: expected add|delete <prefix> dev <alias> [via <gateway>] metric <n>")
	}
	verb, prefix, alias := args[0], args[1], args[3]
	pfx, err := netip.ParsePrefix(prefix)
	if err != nil {
		return fmt.Errorf("iphlpapi route: %w", err)
	}
	var gw netip.Addr
	metric := uint32(0)
	for i := 4; i+1 < len(args); i += 2 {
		switch args[i] {
		case "via":
			gw, err = netip.ParseAddr(args[i+1])
			if err != nil {
				return fmt.Errorf("iphlpapi route: %w", err)
			}
		case "metric":
			m, ok := windowsMetric(args[i+1])
			if !ok {
				return fmt.Errorf("iphlpapi route: bad metric %q", args[i+1])
			}
			metric = m
		}
	}
	luid, err := luidForAlias(alias)
	if err != nil {
		return err
	}
	var row mibIPforwardRow2
	procInitializeIpForwardEntry.Call(uintptr(unsafe.Pointer(&row)))
	row.InterfaceLUID = luid
	row.DestinationPrefix.setPrefix(pfx)
	if gw.IsValid() {
		row.NextHop.setAddr(gw)
	} else if pfx.Addr().Is4() {
		row.NextHop.setAddr(netip.IPv4Unspecified())
	} else {
		row.NextHop.setAddr(netip.IPv6Unspecified())
	}
	row.Metric = metric
	row.Protocol = routeProtocolNetMgmt
	switch verb {
	case "add":
		return classify("route add "+prefix, call(procCreateIpForwardEntry2, uintptr(unsafe.Pointer(&row))))
	case "delete":
		return classify("route delete "+prefix, call(procDeleteIpForwardEntry2, uintptr(unsafe.Pointer(&row))))
	}
	return fmt.Errorf("iphlpapi route: unknown verb %q", verb)
}

// addrCommand: add|delete <prefix> dev <alias>
func addrCommand(args []string) error {
	if len(args) != 4 || args[2] != "dev" {
		return errors.New("iphlpapi addr: expected add|delete <prefix> dev <alias>")
	}
	pfx, err := netip.ParsePrefix(args[1])
	if err != nil {
		return fmt.Errorf("iphlpapi addr: %w", err)
	}
	luid, err := luidForAlias(args[3])
	if err != nil {
		return err
	}
	var row mibUnicastIPAddressRow
	procInitializeUnicastIpAddressEntry.Call(uintptr(unsafe.Pointer(&row)))
	row.Address.setAddr(pfx.Addr())
	row.InterfaceLUID = luid
	row.OnLinkPrefixLength = uint8(pfx.Bits())
	row.DadState = dadStatePreferred
	row.PrefixOrigin = prefixOriginManual
	row.SuffixOrigin = suffixOriginManual
	row.ValidLifetime = lifetimeInfinite
	row.PreferredLifetime = lifetimeInfinite
	switch args[0] {
	case "add":
		return classify("addr add "+args[1], call(procCreateUnicastIpAddressEntry, uintptr(unsafe.Pointer(&row))))
	case "delete":
		return classify("addr delete "+args[1], call(procDeleteUnicastIpAddressEntry, uintptr(unsafe.Pointer(&row))))
	}
	return fmt.Errorf("iphlpapi addr: unknown verb %q", args[0])
}

// ifaceCommand: set <alias> forwarding on|off | set <alias> metric <n>|auto
func ifaceCommand(args []string) error {
	if len(args) != 4 || args[0] != "set" {
		return errors.New("iphlpapi iface: expected set <alias> forwarding on|off, or set <alias> metric <n>|auto")
	}
	luid, err := luidForAlias(args[1])
	if err != nil {
		return err
	}
	var row mibIPInterfaceRow
	row.Family = afInet
	row.InterfaceLUID = luid
	if err := call(procGetIpInterfaceEntry, uintptr(unsafe.Pointer(&row))); err != nil {
		return classify("GetIpInterfaceEntry "+args[1], err)
	}
	// GetIpInterfaceEntry returns 255 in SitePrefixLength for IPv4 and
	// SetIpInterfaceEntry then refuses the row; zero is the documented value
	// to write back.
	row.SitePrefixLength = 0
	switch args[2] {
	case "forwarding":
		row.ForwardingEnabled = args[3] == "on"
	case "metric":
		if args[3] == "auto" {
			row.UseAutomaticMetric = true
		} else {
			m, ok := windowsMetric(args[3])
			if !ok {
				return fmt.Errorf("iphlpapi iface: bad metric %q", args[3])
			}
			row.UseAutomaticMetric = false
			row.Metric = m
		}
	default:
		return fmt.Errorf("iphlpapi iface: unknown setting %q", args[2])
	}
	return classify("SetIpInterfaceEntry "+args[1], call(procSetIpInterfaceEntry, uintptr(unsafe.Pointer(&row))))
}

// ---------------------------------------------------------------------------
// wintun
// ---------------------------------------------------------------------------

func (w *windowsRunner) wintun(args []string) error {
	if len(args) != 2 {
		return errors.New("wintun: expected create <name> or delete <name>")
	}
	name := args[1]
	w.mu.Lock()
	defer w.mu.Unlock()
	switch args[0] {
	case "create":
		if _, ok := w.tuns[name]; ok {
			return fmt.Errorf("wintun create %s: object exists", name)
		}
		if a, err := wintun.OpenAdapter(name); err == nil {
			// Left behind by an earlier process. Adopt it rather than fail:
			// the engine opens by name and does not care who created it.
			w.tuns[name] = a
			return fmt.Errorf("wintun create %s: object exists", name)
		}
		// The same GUID xray-core derives (proxy/tun/tun_windows.go), so the
		// adapter is one and the same whichever side creates it first.
		sum := md5.Sum([]byte(name))
		guid := *(*windows.GUID)(unsafe.Pointer(&sum[0]))
		a, err := wintun.CreateAdapter(name, "Caspian", &guid)
		if err != nil {
			return fmt.Errorf("wintun create %s: %w", name, err)
		}
		w.tuns[name] = a
		return nil
	case "delete":
		a, ok := w.tuns[name]
		if !ok {
			opened, err := wintun.OpenAdapter(name)
			if err != nil {
				return fmt.Errorf("wintun delete %s: not in table", name)
			}
			a = opened
		}
		delete(w.tuns, name)
		if err := a.Close(); err != nil {
			return fmt.Errorf("wintun delete %s: %w", name, err)
		}
		return nil
	}
	return fmt.Errorf("wintun: unknown verb %q", args[0])
}

func (w *windowsRunner) tunLUID(name string) (netLUID, error) {
	w.mu.Lock()
	a, ok := w.tuns[name]
	w.mu.Unlock()
	if ok {
		return netLUID(a.LUID()), nil
	}
	return luidForAlias(name)
}

// ---------------------------------------------------------------------------
// WFP
// ---------------------------------------------------------------------------

func (w *windowsRunner) wfp(args []string, stdin string) error {
	if len(args) != 1 {
		return errors.New("wfp: expected load or flush")
	}
	switch args[0] {
	case "load":
		var fs WindowsFilterSet
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdin)), &fs); err != nil {
			return fmt.Errorf("wfp load: %w", err)
		}
		hot, err := netip.ParsePrefix(fs.Hotspot)
		if err != nil || !hot.Addr().Is4() {
			return fmt.Errorf("wfp load: hotspot subnet %q is not an IPv4 network", fs.Hotspot)
		}
		tun, err := w.tunLUID(fs.Tun)
		if err != nil {
			return err
		}
		return withEngine(func(engine uintptr) error {
			return inTransaction(engine, func() error {
				if err := ensureProviderAndSublayer(engine); err != nil {
					return err
				}
				for _, key := range []windows.GUID{caspianFilterPermitTunV4, caspianFilterBlockV4, caspianFilterBlockV6} {
					if err := deleteFilter(engine, key); err != nil {
						return err
					}
				}
				if fs.Forward != "cut" {
					if err := addFilter(engine, filterSpec{
						key: caspianFilterPermitTunV4, layer: fwpmLayerIPForwardV4, action: fwpActionPermit, weight: 15,
						name:   "Caspian: hotspot clients may be forwarded into the tunnel",
						source: &hot, forwardInterface: uint64(tun),
					}); err != nil {
						return err
					}
				}
				if err := addFilter(engine, filterSpec{
					key: caspianFilterBlockV4, layer: fwpmLayerIPForwardV4, action: fwpActionBlock, weight: 10,
					name:   "Caspian: nothing else from hotspot clients is forwarded",
					source: &hot,
				}); err != nil {
					return err
				}
				return addFilter(engine, filterSpec{
					key: caspianFilterBlockV6, layer: fwpmLayerIPForwardV6, action: fwpActionBlock, weight: 10,
					name: "Caspian: no IPv6 is forwarded while the hotspot is up",
				})
			})
		})
	case "flush":
		return withEngine(func(engine uintptr) error {
			return inTransaction(engine, func() error {
				for _, key := range []windows.GUID{caspianFilterPermitTunV4, caspianFilterBlockV4, caspianFilterBlockV6} {
					if err := deleteFilter(engine, key); err != nil {
						return err
					}
				}
				k := caspianSublayerKey
				if err := call(procFwpmSubLayerDeleteByKey0, engine, uintptr(unsafe.Pointer(&k))); err != nil {
					if e := classify("sublayer", err); !strings.Contains(e.Error(), "not in table") {
						return e
					}
				}
				k = caspianProviderKey
				if err := call(procFwpmProviderDeleteByKey0, engine, uintptr(unsafe.Pointer(&k))); err != nil {
					if e := classify("provider", err); !strings.Contains(e.Error(), "not in table") {
						return e
					}
				}
				return nil
			})
		})
	}
	return fmt.Errorf("wfp: unknown verb %q", args[0])
}

func withEngine(f func(engine uintptr) error) error {
	name, _ := windows.UTF16PtrFromString("Caspian-BYOC")
	desc, _ := windows.UTF16PtrFromString("fail-closed forwarding filters for the hotspot")
	// Not a dynamic session: the filters must outlive this process, so that a
	// crash leaves the box fail-closed rather than open. "wfp flush" removes
	// them, and the journal makes sure it runs.
	session := fwpmSession0{displayData: fwpmDisplayData0{name: name, description: desc}}
	var engine uintptr
	const rpcAuthnDefault = 0xFFFFFFFF
	if err := call(procFwpmEngineOpen0, 0, rpcAuthnDefault, 0, uintptr(unsafe.Pointer(&session)), uintptr(unsafe.Pointer(&engine))); err != nil {
		return classify("FwpmEngineOpen0", err)
	}
	defer procFwpmEngineClose0.Call(engine)
	return f(engine)
}

func inTransaction(engine uintptr, f func() error) error {
	if err := call(procFwpmTransactionBegin0, engine, 0); err != nil {
		return classify("FwpmTransactionBegin0", err)
	}
	if err := f(); err != nil {
		procFwpmTransactionAbort0.Call(engine)
		return err
	}
	if err := call(procFwpmTransactionCommit0, engine); err != nil {
		procFwpmTransactionAbort0.Call(engine)
		return classify("FwpmTransactionCommit0", err)
	}
	return nil
}

func ensureProviderAndSublayer(engine uintptr) error {
	name, _ := windows.UTF16PtrFromString("Caspian-BYOC")
	desc, _ := windows.UTF16PtrFromString("Caspian-BYOC hotspot appliance")
	prov := fwpmProvider0{providerKey: caspianProviderKey, displayData: fwpmDisplayData0{name: name, description: desc}, flags: fwpmProviderFlagPersistent}
	if err := call(procFwpmProviderAdd0, engine, uintptr(unsafe.Pointer(&prov)), 0); err != nil {
		if e := classify("provider", err); !strings.Contains(e.Error(), "object exists") {
			return e
		}
	}
	pk := caspianProviderKey
	sub := fwpmSublayer0{subLayerKey: caspianSublayerKey, displayData: fwpmDisplayData0{name: name, description: desc},
		flags: fwpmSublayerFlagPersistent, providerKey: &pk, weight: 0xFFFF}
	if err := call(procFwpmSubLayerAdd0, engine, uintptr(unsafe.Pointer(&sub)), 0); err != nil {
		if e := classify("sublayer", err); !strings.Contains(e.Error(), "object exists") {
			return e
		}
	}
	return nil
}

func deleteFilter(engine uintptr, key windows.GUID) error {
	if err := call(procFwpmFilterDeleteByKey0, engine, uintptr(unsafe.Pointer(&key))); err != nil {
		if e := classify("filter", err); !strings.Contains(e.Error(), "not in table") {
			return e
		}
	}
	return nil
}

type filterSpec struct {
	key              windows.GUID
	layer            windows.GUID
	action           uint32
	weight           uint8
	name             string
	source           *netip.Prefix
	forwardInterface uint64
}

func addFilter(engine uintptr, spec filterSpec) error {
	name, _ := windows.UTF16PtrFromString(spec.name)
	var conds []fwpmFilterCondition0
	var mask fwpV4AddrAndMask
	if spec.source != nil {
		bits := spec.source.Bits()
		mask = fwpV4AddrAndMask{addr: hostOrder(spec.source.Masked().Addr()), mask: ^uint32(0) << (32 - bits)}
		if bits == 0 {
			mask.mask = 0
		}
		conds = append(conds, fwpmFilterCondition0{fieldKey: fwpmConditionIPSourceAddress, matchType: fwpMatchEqual,
			conditionValue: fwpValue0{kind: fwpV4AddrMask, value: uintptr(unsafe.Pointer(&mask))}})
	}
	iface := spec.forwardInterface
	if iface != 0 {
		conds = append(conds, fwpmFilterCondition0{fieldKey: fwpmConditionIPForwardInterface, matchType: fwpMatchEqual,
			conditionValue: fwpValue0{kind: fwpUint64, value: uintptr(unsafe.Pointer(&iface))}})
	}
	pk := caspianProviderKey
	filter := fwpmFilter0{
		filterKey:   spec.key,
		displayData: fwpmDisplayData0{name: name},
		flags:       fwpmFilterFlagPersistent,
		providerKey: &pk,
		layerKey:    spec.layer,
		subLayerKey: caspianSublayerKey,
		weight:      fwpValue0{kind: fwpUint8, value: uintptr(spec.weight)},
		action:      fwpmAction0{kind: spec.action},
	}
	if len(conds) > 0 {
		filter.numFilterConditions = uint32(len(conds))
		filter.filterCondition = &conds[0]
	}
	var id uint64
	err := call(procFwpmFilterAdd0, engine, uintptr(unsafe.Pointer(&filter)), 0, uintptr(unsafe.Pointer(&id)))
	// Keep the condition values alive across the call.
	_ = mask
	_ = iface
	return classify("filter "+spec.name, err)
}
