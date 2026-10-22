// SPDX-FileCopyrightText: 2026 The SonicRed contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package main

import (
	"os"
	"syscall"
)

// permOthersMask selects the permission bits of group and others.
const permOthersMask = 0o077

// certCacheAccessProblem checks that the certificate cache directory is only accessible by the current user, as it
// contains private keys. It returns a description of the problem, or an empty string if there is none.
func certCacheAccessProblem(info os.FileInfo) string {
	if info.Mode().Perm()&permOthersMask != 0 {
		return "is accessible by other users"
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != os.Geteuid() {
		return "is not owned by the current user"
	}

	return ""
}
