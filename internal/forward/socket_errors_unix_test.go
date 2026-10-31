// Copyright 2026 soulteary
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build unix

package forward

import (
	"errors"
	"syscall"
	"testing"
)

type rawControlFailure struct{}

func (rawControlFailure) Control(func(uintptr)) error    { return syscall.EBADF }
func (rawControlFailure) Read(func(uintptr) bool) error  { return syscall.EBADF }
func (rawControlFailure) Write(func(uintptr) bool) error { return syscall.EBADF }

type syscallFailure struct{}

func (syscallFailure) SyscallConn() (syscall.RawConn, error) { return nil, syscall.EBADF }

type controlFailure struct{}

func (controlFailure) SyscallConn() (syscall.RawConn, error) { return rawControlFailure{}, nil }
func TestSocketControlFailure(t *testing.T) {
	if listenerDualStack(struct{}{}) || listenerDualStack(syscallFailure{}) || listenerDualStack(controlFailure{}) {
		t.Fatal("invalid descriptor reported dual-stack")
	}
	lc := New(Config{ReuseAddr: true}).listenConfig()
	if err := lc.Control("tcp", "", rawControlFailure{}); !errors.Is(err, syscall.EBADF) {
		t.Fatal(err)
	}
}
