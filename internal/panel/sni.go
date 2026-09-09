// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

package panel

import (
	"caspianbyoc.org/caspian/internal/link"
	"caspianbyoc.org/caspian/internal/snispoof"
	"net/http"
)

func (p *Panel) handleSNI(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r)
	name, err := snispoof.NormalizeName(r.PostFormValue("spoof_sni"))
	if err != nil {
		sess.setFlash(Problem{Headline: MsgSNISpoofInvalid}, "")
		p.home(w, r)
		return
	}
	proxy := p.store.Proxy()
	l, _, err := link.Select(proxy.Raw.Reveal(), proxy.Selected)
	if err != nil {
		sess.setFlash(ParseProblem(err), "")
		p.home(w, r)
		return
	}
	if name != "" && !snispoof.SupportsTransport(l.Protocol, l.Network) {
		sess.setFlash(StartProblem(FaultSNISpoofUnsupported), "")
		p.home(w, r)
		return
	}
	if err := p.store.SetSpoofSNI(name); err != nil {
		sess.setFlash(Problem{Headline: MsgSaveConfigFailed, Advice: MsgSaveFailedAdvice}, "")
		p.home(w, r)
		return
	}
	p.afterConfigChange(w, r, sess, MsgSNISpoofSaved, MsgSNISpoofSaved)
}
