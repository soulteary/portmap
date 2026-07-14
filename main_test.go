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

package main

import "testing"

// TestRunValidation 覆盖 run() 中不会真正启动服务的路径：
// 校验阶段提前返回错误，以及 -version 提前返回 nil。
// go/socat 分支会监听端口或执行外部命令，不在单元测试内触发。
func TestRunValidation(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantErr bool
	}{
		{name: "version 提前返回 nil", argv: []string{"-version"}, wantErr: false},

		{name: "端口为 0 非法", argv: []string{"-listen-port", "0"}, wantErr: true},
		{name: "端口超范围非法", argv: []string{"-listen-port", "70000"}, wantErr: true},
		{name: "端口为负非法", argv: []string{"-listen-port", "-1"}, wantErr: true},

		{name: "target 为空非法", argv: []string{"-target", ""}, wantErr: true},

		{name: "未知 proto 非法", argv: []string{"-proto", "sctp"}, wantErr: true},

		{name: "idle-timeout 为负非法", argv: []string{"-idle-timeout", "-1s"}, wantErr: true},
		{name: "max-conns 为负非法", argv: []string{"-max-conns", "-1"}, wantErr: true},
		{name: "dial-timeout 为负非法", argv: []string{"-dial-timeout", "-1s"}, wantErr: true},

		{name: "未知 log-level 非法", argv: []string{"-log-level", "trace"}, wantErr: true},

		// mode 校验位于校验区之后，需保证其它参数合法才能走到 mode 分支。
		{name: "未知 mode 非法", argv: []string{"-mode", "foo"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.argv)
			if (err != nil) != tc.wantErr {
				t.Fatalf("run(%v) 返回 err=%v，期望 wantErr=%v", tc.argv, err, tc.wantErr)
			}
		})
	}
}
