// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 Iman Samizadeh

//go:build unix

package main

import (
	"io/fs"
	"os/user"
	"strconv"
	"syscall"
)

// ownerOf names the user and group that own a file, for the report "caspian
// check" prints.
//
// docs/LAYOUT.md fixes an owner as well as a mode for every path this appliance
// uses, and the ownership is what makes the mode mean anything: /run/caspian at
// 0750 is only a boundary while it is root:caspian. So a report that showed the
// mode and not the owner would show half of the property.
func ownerOf(fi fs.FileInfo) string {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "unknown"
	}
	return nameOrID(user.LookupId, strconv.FormatUint(uint64(st.Uid), 10)) + ":" +
		nameOrGroupID(strconv.FormatUint(uint64(st.Gid), 10))
}

func nameOrID(lookup func(string) (*user.User, error), id string) string {
	if u, err := lookup(id); err == nil {
		return u.Username
	}
	return id
}

func nameOrGroupID(id string) string {
	if g, err := user.LookupGroupId(id); err == nil {
		return g.Name
	}
	return id
}
