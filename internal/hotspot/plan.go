// SPDX-License-Identifier: AGPL-3.0-or-later

package hotspot

// Plan is a fully validated, fully rendered hotspot, ready to start.
//
// Everything that can fail on the input side has already failed by the time a
// Plan exists: the radio constraint is checked, the passphrase is present and
// legal, and both configuration files are rendered. Supervisor.Start therefore
// fails only for reasons that are about the machine, which is what makes the
// failure messages worth showing to a non-technical user.
type Plan struct {
	// AP is the access point configuration as it will be written, including
	// the passphrase that was generated when the caller supplied none.
	AP APConfig

	// DNS is the DHCP and DNS configuration as it will be written.
	DNS DNSConfig

	// Radio is what the caller measured about the radio.
	Radio RadioConstraint

	// HostapdConf and DnsmasqConf are the rendered files.
	HostapdConf string
	DnsmasqConf string

	// PassphraseGenerated is true when no passphrase was supplied and one was
	// made. The panel shows a generated passphrase to the user, since nobody
	// else knows it.
	PassphraseGenerated bool
}

// NewPlan validates the inputs, fills in a passphrase if there is none, and
// renders both configuration files.
//
// Pure apart from reading the system's random source when it has to generate a
// passphrase.
func NewPlan(ap APConfig, dns DNSConfig, radio RadioConstraint) (Plan, error) {
	ap, generated, err := EnsurePassphrase(ap)
	if err != nil {
		return Plan{}, err
	}
	if err := ap.Validate(); err != nil {
		return Plan{}, err
	}
	// The radio check comes after the config is known good, so that a user
	// with two problems is told about the one they can act on rather than
	// about a channel they never chose.
	if err := radio.Check(ap); err != nil {
		return Plan{}, err
	}
	if dns.Interface == "" {
		// The DHCP and DNS server serve the access point, so they are on the
		// same interface by definition. Defaulting it here removes a way for
		// the caller to set up a hotspot that hands out no addresses.
		dns.Interface = ap.Interface
	}
	if dns.ServiceUser == "" {
		// The documented account from docs/LAYOUT.md. Defaulted here, in the
		// one place that builds a whole hotspot, rather than inside the
		// renderer: a caller that renders a config directly has to state the
		// account, so a second code path cannot quietly disagree with the
		// installer about who owns the lease directory.
		dns.ServiceUser = DefaultServiceUser
	}
	if dns.ServiceGroup == "" {
		dns.ServiceGroup = DefaultServiceGroup
	}
	if err := dns.Validate(); err != nil {
		return Plan{}, err
	}

	hconf, err := RenderHostapd(ap)
	if err != nil {
		return Plan{}, err
	}
	dconf, err := RenderDnsmasq(dns)
	if err != nil {
		return Plan{}, err
	}

	return Plan{
		AP:                  ap,
		DNS:                 dns,
		Radio:               radio,
		HostapdConf:         hconf,
		DnsmasqConf:         dconf,
		PassphraseGenerated: generated,
	}, nil
}
