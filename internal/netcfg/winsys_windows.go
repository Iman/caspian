// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build windows && (amd64 || arm64)

package netcfg

import (
	"encoding/binary"
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The IP Helper API and Windows Filtering Platform surface the Windows runner
// needs, as lazily bound procedures and the structures they take.
//
// Struct layouts are the 64-bit ones from WireGuard's MIT winipcfg and
// firewall packages (golang.zx2c4.com/wireguard/windows), copied field for
// field on 2026-09-03 rather than typed from memory, because a wrong offset
// here is a route added to the wrong adapter and nothing that reports it.
// The build tag says so: 32-bit Windows is not built.

var (
	modiphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
	modfwpuclnt = windows.NewLazySystemDLL("fwpuclnt.dll")

	procGetIfTable2Ex                   = modiphlpapi.NewProc("GetIfTable2Ex")
	procFreeMibTable                    = modiphlpapi.NewProc("FreeMibTable")
	procGetIpForwardTable2              = modiphlpapi.NewProc("GetIpForwardTable2")
	procInitializeIpForwardEntry        = modiphlpapi.NewProc("InitializeIpForwardEntry")
	procCreateIpForwardEntry2           = modiphlpapi.NewProc("CreateIpForwardEntry2")
	procDeleteIpForwardEntry2           = modiphlpapi.NewProc("DeleteIpForwardEntry2")
	procInitializeUnicastIpAddressEntry = modiphlpapi.NewProc("InitializeUnicastIpAddressEntry")
	procCreateUnicastIpAddressEntry     = modiphlpapi.NewProc("CreateUnicastIpAddressEntry")
	procDeleteUnicastIpAddressEntry     = modiphlpapi.NewProc("DeleteUnicastIpAddressEntry")
	procGetIpInterfaceEntry             = modiphlpapi.NewProc("GetIpInterfaceEntry")
	procSetIpInterfaceEntry             = modiphlpapi.NewProc("SetIpInterfaceEntry")
	procConvertInterfaceAliasToLuid     = modiphlpapi.NewProc("ConvertInterfaceAliasToLuid")
	procConvertInterfaceLuidToAlias     = modiphlpapi.NewProc("ConvertInterfaceLuidToAlias")

	procFwpmEngineOpen0          = modfwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0         = modfwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmTransactionBegin0    = modfwpuclnt.NewProc("FwpmTransactionBegin0")
	procFwpmTransactionCommit0   = modfwpuclnt.NewProc("FwpmTransactionCommit0")
	procFwpmTransactionAbort0    = modfwpuclnt.NewProc("FwpmTransactionAbort0")
	procFwpmProviderAdd0         = modfwpuclnt.NewProc("FwpmProviderAdd0")
	procFwpmProviderDeleteByKey0 = modfwpuclnt.NewProc("FwpmProviderDeleteByKey0")
	procFwpmSubLayerAdd0         = modfwpuclnt.NewProc("FwpmSubLayerAdd0")
	procFwpmSubLayerDeleteByKey0 = modfwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFwpmFilterAdd0           = modfwpuclnt.NewProc("FwpmFilterAdd0")
	procFwpmFilterDeleteByKey0   = modfwpuclnt.NewProc("FwpmFilterDeleteByKey0")
)

// ---------------------------------------------------------------------------
// IP Helper
// ---------------------------------------------------------------------------

type netLUID uint64

type addressFamily uint16

const (
	afUnspec addressFamily = windows.AF_UNSPEC
	afInet   addressFamily = windows.AF_INET
	afInet6  addressFamily = windows.AF_INET6
)

// rawSockaddrInet is SOCKADDR_INET: a family and 26 bytes covering the larger
// of sockaddr_in and sockaddr_in6.
type rawSockaddrInet struct {
	Family addressFamily
	data   [26]byte
}

func (a *rawSockaddrInet) setAddr(addr netip.Addr) {
	*a = rawSockaddrInet{}
	if addr.Is4() {
		a4 := (*windows.RawSockaddrInet4)(unsafe.Pointer(a))
		a4.Family = windows.AF_INET
		a4.Addr = addr.As4()
		return
	}
	if addr.Is6() {
		a6 := (*windows.RawSockaddrInet6)(unsafe.Pointer(a))
		a6.Family = windows.AF_INET6
		a6.Addr = addr.As16()
	}
}

func (a *rawSockaddrInet) addr() netip.Addr {
	switch a.Family {
	case afInet:
		return netip.AddrFrom4((*windows.RawSockaddrInet4)(unsafe.Pointer(a)).Addr)
	case afInet6:
		return netip.AddrFrom16((*windows.RawSockaddrInet6)(unsafe.Pointer(a)).Addr)
	}
	return netip.Addr{}
}

type ipAddressPrefix struct {
	RawPrefix    rawSockaddrInet
	PrefixLength uint8
	_            [2]byte
}

func (p *ipAddressPrefix) setPrefix(pfx netip.Prefix) {
	p.RawPrefix.setAddr(pfx.Masked().Addr())
	p.PrefixLength = uint8(pfx.Bits())
}

type routeProtocol uint32

const routeProtocolNetMgmt routeProtocol = 3

type mibIPforwardRow2 struct {
	InterfaceLUID        netLUID
	InterfaceIndex       uint32
	DestinationPrefix    ipAddressPrefix
	NextHop              rawSockaddrInet
	SitePrefixLength     uint8
	ValidLifetime        uint32
	PreferredLifetime    uint32
	Metric               uint32
	Protocol             routeProtocol
	Loopback             bool
	AutoconfigureAddress bool
	Publish              bool
	Immortal             bool
	Age                  uint32
	Origin               uint32
}

type mibIPforwardTable2 struct {
	numEntries uint32
	_          [4]byte
	table      [1]mibIPforwardRow2
}

func (t *mibIPforwardTable2) rows() []mibIPforwardRow2 {
	return unsafe.Slice(&t.table[0], t.numEntries)
}

type mibUnicastIPAddressRow struct {
	Address            rawSockaddrInet
	InterfaceLUID      netLUID
	InterfaceIndex     uint32
	PrefixOrigin       uint32
	SuffixOrigin       uint32
	ValidLifetime      uint32
	PreferredLifetime  uint32
	OnLinkPrefixLength uint8
	SkipAsSource       bool
	DadState           uint32
	ScopeID            uint32
	CreationTimeStamp  int64
}

const (
	dadStatePreferred  uint32 = 4
	prefixOriginManual uint32 = 1
	suffixOriginManual uint32 = 1
	lifetimeInfinite   uint32 = 0xffffffff
)

const scopeLevelCount = 16

type mibIPInterfaceRow struct {
	Family                               addressFamily
	InterfaceLUID                        netLUID
	InterfaceIndex                       uint32
	MaxReassemblySize                    uint32
	InterfaceIdentifier                  uint64
	MinRouterAdvertisementInterval       uint32
	MaxRouterAdvertisementInterval       uint32
	AdvertisingEnabled                   bool
	ForwardingEnabled                    bool
	WeakHostSend                         bool
	WeakHostReceive                      bool
	UseAutomaticMetric                   bool
	UseNeighborUnreachabilityDetection   bool
	ManagedAddressConfigurationSupported bool
	OtherStatefulConfigurationSupported  bool
	AdvertiseDefaultRoute                bool
	RouterDiscoveryBehavior              int32
	DadTransmits                         uint32
	BaseReachableTime                    uint32
	RetransmitTime                       uint32
	PathMTUDiscoveryTimeout              uint32
	LinkLocalAddressBehavior             int32
	LinkLocalAddressTimeout              uint32
	ZoneIndices                          [scopeLevelCount]uint32
	SitePrefixLength                     uint32
	Metric                               uint32
	NLMTU                                uint32
	Connected                            bool
	SupportsWakeUpPatterns               bool
	SupportsNeighborDiscovery            bool
	SupportsRouterDiscovery              bool
	ReachableTime                        uint32
	TransmitOffload                      uint8
	ReceiveOffload                       uint8
	DisableDefaultRoutes                 bool
}

const (
	ifMaxStringSize        = 256
	ifMaxPhysAddressLength = 32
)

// mibIfRow2 is MIB_IF_ROW2: the interface table entry with alias, type and
// operational status. Only the fields the runner reads are named.
type mibIfRow2 struct {
	InterfaceLUID               netLUID
	InterfaceIndex              uint32
	InterfaceGUID               windows.GUID
	alias                       [ifMaxStringSize + 1]uint16
	description                 [ifMaxStringSize + 1]uint16
	physicalAddressLength       uint32
	physicalAddress             [ifMaxPhysAddressLength]byte
	permanentPhysicalAddress    [ifMaxPhysAddressLength]byte
	MTU                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags uint8
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGUID                 windows.GUID
	ConnectionType              uint32
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

func (r *mibIfRow2) aliasString() string       { return windows.UTF16ToString(r.alias[:]) }
func (r *mibIfRow2) descriptionString() string { return windows.UTF16ToString(r.description[:]) }

type mibIfTable2 struct {
	numEntries uint32
	_          [4]byte
	table      [1]mibIfRow2
}

func (t *mibIfTable2) rows() []mibIfRow2 { return unsafe.Slice(&t.table[0], t.numEntries) }

const (
	ifTypeEthernet    = 6
	ifTypeLoopback    = 24
	ifTypePropVirtual = 53
	ifTypeIEEE80211   = 71
	ifTypeTunnel      = 131
	ifOperStatusUp    = 1
)

// mibIfFilterInterface is the FilterInterface bit of MIB_IF_ROW2's
// InterfaceAndOperStatusFlags. MEASURED against a live table (see
// readInventory).
const mibIfFilterInterface = 0x02

// Windows error codes the runner turns into the idempotence markers the
// shared Applier already understands.
const (
	errorObjectAlreadyExists = 5010
	errorNotFound            = 1168
	fwpEAlreadyExists        = 0x80320009
	fwpEFilterNotFound       = 0x80320003
	fwpESublayerNotFound     = 0x80320007
	fwpEProviderNotFound     = 0x80320005
	fwpEInUse                = 0x8032000A
)

func call(p *windows.LazyProc, args ...uintptr) error {
	r, _, _ := p.Call(args...)
	if r != 0 {
		return windows.Errno(r)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Windows Filtering Platform
// ---------------------------------------------------------------------------

type fwpmDisplayData0 struct {
	name        *uint16
	description *uint16
}

type fwpByteBlob struct {
	size uint32
	data *uint8
}

type fwpmSession0 struct {
	sessionKey           windows.GUID
	displayData          fwpmDisplayData0
	flags                uint32
	txnWaitTimeoutInMSec uint32
	processID            uint32
	sid                  *windows.SID
	username             *uint16
	kernelMode           uint8
}

type fwpmProvider0 struct {
	providerKey  windows.GUID
	displayData  fwpmDisplayData0
	flags        uint32
	providerData fwpByteBlob
	serviceName  *uint16
}

type fwpmSublayer0 struct {
	subLayerKey  windows.GUID
	displayData  fwpmDisplayData0
	flags        uint32
	providerKey  *windows.GUID
	providerData fwpByteBlob
	weight       uint16
}

type fwpValue0 struct {
	kind  uint32
	value uintptr
}

const (
	fwpEmpty      uint32 = 0
	fwpUint8      uint32 = 1
	fwpUint64     uint32 = 4
	fwpV4AddrMask uint32 = 0x100
)

type fwpV4AddrAndMask struct {
	addr uint32
	mask uint32
}

type fwpmFilterCondition0 struct {
	fieldKey       windows.GUID
	matchType      uint32
	conditionValue fwpValue0
}

const fwpMatchEqual uint32 = 0

type fwpmAction0 struct {
	kind       uint32
	filterType windows.GUID
}

const (
	fwpActionBlock  uint32 = 0x00000001 | 0x00001000
	fwpActionPermit uint32 = 0x00000002 | 0x00001000
)

const (
	fwpmFilterFlagPersistent   uint32 = 0x00000001
	fwpmSublayerFlagPersistent uint32 = 0x00000001
	fwpmProviderFlagPersistent uint32 = 0x00000001
)

// fwpmFilter0 is the 64-bit FWPM_FILTER0, with the one layout correction
// WireGuard documents after the action.
type fwpmFilter0 struct {
	filterKey           windows.GUID
	displayData         fwpmDisplayData0
	flags               uint32
	providerKey         *windows.GUID
	providerData        fwpByteBlob
	layerKey            windows.GUID
	subLayerKey         windows.GUID
	weight              fwpValue0
	numFilterConditions uint32
	filterCondition     *fwpmFilterCondition0
	action              fwpmAction0
	_                   [4]byte
	providerContextKey  windows.GUID
	reserved            *windows.GUID
	filterID            uint64
	effectiveWeight     fwpValue0
}

// Layer and condition identifiers, from Microsoft's fwpmu.h by way of the
// generated tables in tailscale/wf and WireGuard's firewall package.
var (
	fwpmLayerIPForwardV4 = windows.GUID{Data1: 0xa82acc24, Data2: 0x4ee1, Data3: 0x4ee1,
		Data4: [8]byte{0xb4, 0x65, 0xfd, 0x1d, 0x25, 0xcb, 0x10, 0xa4}}
	fwpmLayerIPForwardV6 = windows.GUID{Data1: 0x7b964818, Data2: 0x19c7, Data3: 0x493a,
		Data4: [8]byte{0xb7, 0x1f, 0x83, 0x2c, 0x36, 0x84, 0xd2, 0x8c}}
	fwpmConditionIPSourceAddress = windows.GUID{Data1: 0xae96897e, Data2: 0x2e94, Data3: 0x4bc9,
		Data4: [8]byte{0xb3, 0x13, 0xb2, 0x7e, 0xe8, 0x0e, 0x57, 0x4d}}
	fwpmConditionIPForwardInterface = windows.GUID{Data1: 0x1076b8a5, Data2: 0x6323, Data3: 0x4c5e,
		Data4: [8]byte{0x98, 0x10, 0xe8, 0xd3, 0xfc, 0x9e, 0x61, 0x36}}
)

// This appliance's own WFP objects. Fixed keys, so that a later process can
// remove what an earlier one added by key, and so that "wfp load" twice is an
// update and not a second set.
var (
	caspianProviderKey = windows.GUID{Data1: 0x6c2a5f10, Data2: 0x9b3e, Data3: 0x4d7a,
		Data4: [8]byte{0x8c, 0x41, 0x1a, 0x5e, 0x2d, 0x0b, 0x7f, 0x01}}
	caspianSublayerKey = windows.GUID{Data1: 0x6c2a5f10, Data2: 0x9b3e, Data3: 0x4d7a,
		Data4: [8]byte{0x8c, 0x41, 0x1a, 0x5e, 0x2d, 0x0b, 0x7f, 0x02}}
	caspianFilterPermitTunV4 = windows.GUID{Data1: 0x6c2a5f10, Data2: 0x9b3e, Data3: 0x4d7a,
		Data4: [8]byte{0x8c, 0x41, 0x1a, 0x5e, 0x2d, 0x0b, 0x7f, 0x11}}
	caspianFilterBlockV4 = windows.GUID{Data1: 0x6c2a5f10, Data2: 0x9b3e, Data3: 0x4d7a,
		Data4: [8]byte{0x8c, 0x41, 0x1a, 0x5e, 0x2d, 0x0b, 0x7f, 0x12}}
	caspianFilterBlockV6 = windows.GUID{Data1: 0x6c2a5f10, Data2: 0x9b3e, Data3: 0x4d7a,
		Data4: [8]byte{0x8c, 0x41, 0x1a, 0x5e, 0x2d, 0x0b, 0x7f, 0x13}}
)

// hostOrder returns the IPv4 address as the UINT32 WFP wants: host byte order.
func hostOrder(a netip.Addr) uint32 {
	b := a.As4()
	return binary.BigEndian.Uint32(b[:])
}
