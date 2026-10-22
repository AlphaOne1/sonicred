// SPDX-FileCopyrightText: 2026 The SonicRed contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !unix

package main

import "os"

// certCacheAccessProblem is not implemented on non-unix systems, as they do not report meaningful permission bits.
func certCacheAccessProblem(_ os.FileInfo) string {
	return ""
}
