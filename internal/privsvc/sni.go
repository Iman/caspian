// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package privsvc

import (
	"caspianbyoc.org/caspian/internal/engine"
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/netcfg"
	"caspianbyoc.org/caspian/internal/panel"
	"caspianbyoc.org/caspian/internal/snispoof"
	"errors"
	"net/netip"
)

// SNIForwarder is the service-owned loopback endpoint and its cleanup handle.
type SNIForwarder interface {
	Addr() netip.AddrPort
	Close() error
}

// SNIStarter opens the optional packet forwarder for a resolved proxy endpoint.
type SNIStarter func(netip.AddrPort, string, snispoof.Options) (SNIForwarder, error)

func validateSpoof(l *link.Link, name string, splitting ...bool) error {
	split := false
	for _, enabled := range splitting {
		split = split || enabled
	}
	normalized, err := snispoof.NormalizeName(name)
	if err != nil || name != normalized {
		return fail("SNI spoofing", panel.FaultSNISpoofInvalid, snispoof.ErrName)
	}
	if ((name != "" || split) && !snispoof.SupportsTransport(l.Protocol, l.Network)) || (split && l.Security != link.SecurityTLS) {
		return fail("SNI spoofing", panel.FaultSNISpoofUnsupported, snispoof.ErrUnsupported)
	}
	return nil
}

func (s *Service) spoofDocument(l *link.Link, req panel.StartRequest, plan *netcfg.Plan, opts netcfg.Options) ([]byte, error) {
	var remote netip.Addr
	for _, addr := range plan.ServerAddr {
		if addr.Is4() {
			remote = addr
			break
		}
	}
	if !remote.IsValid() {
		return nil, fail("SNI spoofing", panel.FaultSNISpoofUnsupported, snispoof.ErrUnsupported)
	}
	f, err := s.cfg.StartSNI(netip.AddrPortFrom(remote, l.Port), plan.Uplink, snispoof.Options{FakeSNI: req.SpoofSNI, TCPSplit: req.TCPSplit, TLSRecordSplit: req.TLSRecordSplit})
	if errors.Is(err, snispoof.ErrUnsupported) {
		return nil, fail("SNI spoofing", panel.FaultSNISpoofUnsupported, snispoof.ErrUnsupported)
	}
	if err != nil {
		return nil, fail("SNI spoofing", panel.FaultSNISpoofUnavailable, snispoof.ErrUnavailable)
	}
	s.sniForwarder = f
	forwarded, err := l.ThroughLoopback(f.Addr())
	if err != nil {
		return nil, fail("SNI spoofing", panel.FaultSNISpoofUnavailable, snispoof.ErrUnavailable)
	}
	doc, err := s.engineDocument(forwarded, req, opts)
	if err != nil {
		return nil, err
	}
	if err := engine.Validate(doc); err != nil {
		return nil, fail("SNI spoofing", panel.FaultEngineRejectedConfig, err)
	}
	return doc, nil
}
