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
	"syscall"

	"golang.org/x/sys/unix"
)

// setReuseAddr 在类 Unix 平台上对给定 fd 设置 SO_REUSEADDR。
func setReuseAddr(fd uintptr) error {
	return unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
}

// listenerDualStack reports whether an IPv6 listener also accepts IPv4.
func listenerDualStack(conn any) bool {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return false
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return false
	}
	var value int
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		value, sockErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_V6ONLY)
	}); err != nil || sockErr != nil {
		return false
	}
	return value == 0
}
