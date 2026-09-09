// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh
//
// The pasted texts the entry scenarios put in front of the panel.
//
// A provider's subscription reaches the box as one text holding several links.
// The three below are three protocols at three hosts with three names, so a
// page or a document naming the wrong one is caught by its host, its protocol
// and its name. Every host is under .invalid, which RFC 6761 reserves so that
// nothing can ever resolve it, and every credential is invented and has never
// been a working one.
//
// The same fixture exists in bdd/api/features/support/entries.js. The two
// suites do not share files (each has its own world and hooks), so it is
// repeated rather than reached across.

'use strict';

const FAKE_UUID = '11111111-2222-4333-8444-555555555555';
const FAKE_SNI = 'www.fake-front.invalid';

const HOST_A = 'a.example.invalid';
const HOST_B = 'b.example.invalid';
const HOST_C = 'c.example.invalid';

// The names the three entries carry, in list order. Steps that check no name
// leaked read this list rather than repeating the words.
const ENTRY_NAMES = ['alpha', 'beta', 'gamma'];

function entryLink(protocol, host, name) {
  switch (protocol) {
    case 'vless':
      return 'vless://' + FAKE_UUID + '@' + host + ':443?security=tls&type=raw&sni=' + FAKE_SNI + '#' + name;
    case 'trojan':
      return 'trojan://not-a-real-trojan-password@' + host + ':443?security=tls&type=raw&sni=' + FAKE_SNI + '#' + name;
    case 'ss':
      return 'ss://' + Buffer.from('aes-256-gcm:not-a-real-shadowsocks-password').toString('base64url') +
        '@' + host + ':8388#' + name;
    default:
      throw new Error('entryLink: unknown protocol ' + protocol);
  }
}

// threeEntries is what a provider's subscription looks like once pasted.
function threeEntries() {
  return [
    entryLink('vless', HOST_A, ENTRY_NAMES[0]),
    entryLink('trojan', HOST_B, ENTRY_NAMES[1]),
    entryLink('ss', HOST_C, ENTRY_NAMES[2]),
  ].join('\n');
}

// oneEntry is a single link, which draws no list.
function oneEntry() {
  return entryLink('vless', HOST_A, ENTRY_NAMES[0]);
}

// brokenLine is a link the vendored parser drops without a word: the port does
// not fit in sixteen bits (third_party/libxray-share/parse_share.go, the port
// check in the line loop).
function brokenLine() {
  return 'vless://' + FAKE_UUID + '@' + HOST_B + ':99999?security=tls#broken';
}

function threeEntriesAndABrokenLine() {
  return [
    entryLink('vless', HOST_A, ENTRY_NAMES[0]),
    brokenLine(),
    entryLink('trojan', HOST_B, ENTRY_NAMES[1]),
    entryLink('ss', HOST_C, ENTRY_NAMES[2]),
  ].join('\n');
}

module.exports = { ENTRY_NAMES, threeEntries, oneEntry, threeEntriesAndABrokenLine };
