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

package socat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/portmap/internal/i18n"
)

func TestArgsTCP(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"TCP-LISTEN:22,fork,reuseaddr", "TCP:127.0.0.1:2222"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

func TestArgsWithListenHost(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, ListenHost: "127.0.0.1"}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr", "TCP:127.0.0.1:2222"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

// TestArgsEmptyListenHost 验证 ListenHost 为空（含仅空白）时输出与不带 host 完全一致。
func TestArgsEmptyListenHost(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, ListenHost: "  "}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"TCP-LISTEN:22,fork,reuseaddr", "TCP:127.0.0.1:2222"}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

func TestArgsUDP(t *testing.T) {
	o := Options{ListenPort: 53, Target: "127.0.0.1:5353", Fork: true, ReuseAddr: true, Proto: "udp"}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"UDP-LISTEN:53,fork,reuseaddr", "UDP:127.0.0.1:5353"}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

func TestArgsInvalidProto(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Proto: "sctp"}
	if _, err := o.Args(); err == nil {
		t.Fatal("expected error for invalid proto")
	}
}

func TestArgsInvalidPort(t *testing.T) {
	o := Options{ListenPort: 0, Target: "127.0.0.1:2222"}
	if _, err := o.Args(); err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestArgsEmptyTarget(t *testing.T) {
	o := Options{ListenPort: 22, Target: "  "}
	if _, err := o.Args(); err == nil {
		t.Fatal("expected error for empty target")
	}
}

// TestStringReusesArgs 验证 String 与 Args 输出一致（单一来源）。
func TestStringReusesArgs(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true}
	if got, want := o.String(), "socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}

	sudo := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, Sudo: true}
	if got, want := sudo.String(), "sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}

	udp := Options{ListenPort: 53, Target: "127.0.0.1:5353", Fork: true, ReuseAddr: true, Proto: "udp"}
	if got, want := udp.String(), "socat UDP-LISTEN:53,fork,reuseaddr UDP:127.0.0.1:5353"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}

	host := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, ListenHost: "127.0.0.1"}
	if got, want := host.String(), "socat TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr TCP:127.0.0.1:2222"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}
}

func TestStringInvalid(t *testing.T) {
	i18n.SetLang(i18n.English)
	o := Options{ListenPort: -1, Target: "x"}
	if got := o.String(); got != "<invalid socat options>" {
		t.Fatalf("String()=%q, want invalid marker", got)
	}
}

// TestArgsProtoDefault 验证空 Proto 默认按 TCP 处理，大小写不敏感。
func TestArgsProtoDefault(t *testing.T) {
	for _, proto := range []string{"", "TCP", "Tcp", "  udp  ", "UDP"} {
		o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Proto: proto}
		args, err := o.Args()
		if err != nil {
			t.Fatalf("Args(proto=%q): %v", proto, err)
		}
		wantScheme := "TCP"
		if strings.EqualFold(strings.TrimSpace(proto), "udp") {
			wantScheme = "UDP"
		}
		if !strings.HasPrefix(args[0], wantScheme+"-LISTEN") {
			t.Fatalf("proto=%q -> %q，期望以 %s-LISTEN 开头", proto, args[0], wantScheme)
		}
	}
}

// TestArgsPortUpperBound 验证端口上界（65535 合法、65536 非法）。
func TestArgsPortUpperBound(t *testing.T) {
	if _, err := (Options{ListenPort: 65535, Target: "x:1"}).Args(); err != nil {
		t.Fatalf("端口 65535 应合法: %v", err)
	}
	if _, err := (Options{ListenPort: 65536, Target: "x:1"}).Args(); err == nil {
		t.Fatal("端口 65536 应非法")
	}
}

// stubSocat 在临时目录放置一个名为 name 的可执行脚本，并把该目录加到 PATH 前部，
// 使 exec.LookPath 能找到它。返回清理函数。
func stubSocat(t *testing.T, name string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("写入桩程序失败: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestCommandBuildsExecCmd 验证 Command 在 socat 可用时正确构造 *exec.Cmd。
func TestCommandBuildsExecCmd(t *testing.T) {
	stubSocat(t, "socat")
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true}
	cmd, err := o.Command(context.Background())
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if filepath.Base(cmd.Path) != "socat" {
		t.Fatalf("cmd.Path=%q，期望 socat", cmd.Path)
	}
	// Args[0] 为程序名，其后应是 Args() 的输出。
	if len(cmd.Args) != 3 {
		t.Fatalf("cmd.Args=%v，期望 3 个元素", cmd.Args)
	}
	if cmd.Args[1] != "TCP-LISTEN:22,fork,reuseaddr" {
		t.Fatalf("监听参数=%q", cmd.Args[1])
	}
}

// TestCommandSudoPrefix 验证 Sudo 为 true 时程序名为 sudo，且首个参数为 socat。
func TestCommandSudoPrefix(t *testing.T) {
	stubSocat(t, "sudo")
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Sudo: true}
	cmd, err := o.Command(context.Background())
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if filepath.Base(cmd.Path) != "sudo" {
		t.Fatalf("cmd.Path=%q，期望 sudo", cmd.Path)
	}
	if len(cmd.Args) < 2 || cmd.Args[1] != "socat" {
		t.Fatalf("sudo 首参数应为 socat: %v", cmd.Args)
	}
}

// TestCommandInvalidArgs 验证参数非法时 Command 直接返回错误（不查找二进制）。
func TestCommandInvalidArgs(t *testing.T) {
	o := Options{ListenPort: 0, Target: "x"}
	if _, err := o.Command(context.Background()); err == nil {
		t.Fatal("非法参数应返回错误")
	}
}

// TestCommandNotFound 验证 socat 不存在于 PATH 时返回错误。
func TestCommandNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222"}
	if _, err := o.Command(context.Background()); err == nil {
		t.Fatal("socat 不存在时应返回错误")
	}
}

// TestRunExecutesStub 验证 Run 能执行桩 socat 并成功返回。
func TestRunExecutesStub(t *testing.T) {
	stubSocat(t, "socat")
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222"}
	if err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestRunInvalidReturnsError 验证 Run 在参数非法时返回错误。
func TestRunInvalidReturnsError(t *testing.T) {
	o := Options{ListenPort: -1, Target: "x"}
	if err := o.Run(context.Background()); err == nil {
		t.Fatal("非法参数 Run 应返回错误")
	}
}

// TestSetGracefulCancel 验证类 Unix 平台设置了 Cancel 回调（发送 SIGTERM）。
func TestSetGracefulCancel(t *testing.T) {
	stubSocat(t, "socat")
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222"}
	cmd, err := o.Command(context.Background())
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if cmd.Cancel == nil {
		t.Fatal("期望设置了 Cancel 回调以优雅终止")
	}
}
