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

//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

// systemLocale 在 Windows 上调用 GetUserDefaultLocaleName 获取用户区域名，
// 例如 "zh-CN"、"ja-JP"。失败时返回空字符串。
func systemLocale() string {
	kernel32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return ""
	}
	proc, err := kernel32.FindProc("GetUserDefaultLocaleName")
	if err != nil {
		return ""
	}
	// LOCALE_NAME_MAX_LENGTH = 85（含结尾 NUL）。
	buf := make([]uint16, 85)
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:ret])
}
