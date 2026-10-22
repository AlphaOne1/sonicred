// SPDX-FileCopyrightText: 2026 The SonicRed contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build unix

package main

import (
	"io/fs"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// fakeDirInfo is a directory info with the given permissions and owner.
type fakeDirInfo struct {
	perm fs.FileMode
	uid  uint32
}

func (f fakeDirInfo) Name() string       { return "cache" }
func (f fakeDirInfo) Size() int64        { return 0 }
func (f fakeDirInfo) Mode() fs.FileMode  { return fs.ModeDir | f.perm }
func (f fakeDirInfo) ModTime() time.Time { return time.Time{} }
func (f fakeDirInfo) IsDir() bool        { return true }
func (f fakeDirInfo) Sys() any           { return &syscall.Stat_t{Uid: f.uid} }

func TestCertCacheAccessProblem(t *testing.T) {
	t.Parallel()

	self := uint32(os.Geteuid()) //nolint:gosec // uids are not negative

	tests := []struct {
		info        fakeDirInfo
		wantProblem bool
	}{
		{info: fakeDirInfo{perm: 0o700, uid: self}, wantProblem: false},
		{info: fakeDirInfo{perm: 0o750, uid: self}, wantProblem: true},
		{info: fakeDirInfo{perm: 0o707, uid: self}, wantProblem: true},
		{info: fakeDirInfo{perm: 0o700, uid: self + 1}, wantProblem: true},
	}

	for _, test := range tests {
		problem := certCacheAccessProblem(test.info)
		assert.Equal(t, test.wantProblem, problem != "", "perm %v, uid %d: %q", test.info.perm, test.info.uid, problem)
	}
}
