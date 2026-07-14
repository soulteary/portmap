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

// Package socat 提供调用系统 socat 命令的 fallback 实现，
// 用于在需要时直接复用本机的 socat 二进制完成端口转发。
package socat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/soulteary/portmap/internal/i18n"
)

// Options 描述生成 socat 命令所需的参数。
type Options struct {
	// ListenPort 是本地监听端口。
	ListenPort int
	// Target 是转发目标地址，例如 "127.0.0.1:2222"。
	Target string
	// Proto 是使用的协议，"tcp"（默认）或 "udp"。
	Proto string
	// Fork 对应 socat 的 fork 选项。
	Fork bool
	// ReuseAddr 对应 socat 的 reuseaddr 选项。
	ReuseAddr bool
	// ListenHost 为非空时在监听地址上追加 bind=<host>，
	// 使 socat 仅绑定该地址（等价 go 模式的 -listen-host）。
	ListenHost string
	// Sudo 为 true 时在命令前加 sudo（监听 1024 以下端口通常需要）。
	Sudo bool
}

// proto 返回规范化后的协议名（tcp/udp），默认 tcp。
func (o Options) proto() (string, error) {
	p := strings.ToLower(strings.TrimSpace(o.Proto))
	switch p {
	case "", "tcp":
		return "tcp", nil
	case "udp":
		return "udp", nil
	default:
		return "", errors.New(i18n.T(i18n.KeyErrSocatProto, o.Proto))
	}
}

// Args 根据 Options 构造 socat 的命令行参数（不含程序名）。
//
// 例如返回：
//
//	["TCP-LISTEN:22,fork,reuseaddr", "TCP:127.0.0.1:2222"]
func (o Options) Args() ([]string, error) {
	if o.ListenPort <= 0 || o.ListenPort > 65535 {
		return nil, errors.New(i18n.T(i18n.KeyErrSocatPort, o.ListenPort))
	}
	if strings.TrimSpace(o.Target) == "" {
		return nil, errors.New(i18n.T(i18n.KeyErrSocatTarget))
	}
	proto, err := o.proto()
	if err != nil {
		return nil, err
	}
	scheme := strings.ToUpper(proto)

	listen := fmt.Sprintf("%s-LISTEN:%d", scheme, o.ListenPort)
	if h := strings.TrimSpace(o.ListenHost); h != "" {
		listen += ",bind=" + h
	}
	if o.Fork {
		listen += ",fork"
	}
	if o.ReuseAddr {
		listen += ",reuseaddr"
	}
	return []string{listen, scheme + ":" + o.Target}, nil
}

// Command 构造可执行的 *exec.Cmd（包含可选的 sudo 前缀），
// 并将子进程的标准输入/输出/错误绑定到当前进程。
func (o Options) Command(ctx context.Context) (*exec.Cmd, error) {
	socatArgs, err := o.Args()
	if err != nil {
		return nil, err
	}

	name := "socat"
	args := socatArgs
	if o.Sudo {
		name = "sudo"
		args = append([]string{"socat"}, socatArgs...)
	}

	if _, err := exec.LookPath(name); err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrSocatNotFound), name, err)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setGracefulCancel(cmd)
	return cmd, nil
}

// String 返回等价的可读命令行，便于日志打印。命令行由单一来源 Args 生成。
func (o Options) String() string {
	args, err := o.Args()
	if err != nil {
		return i18n.T(i18n.KeyErrSocatInvalidStr)
	}
	prefix := "socat"
	if o.Sudo {
		prefix = "sudo socat"
	}
	return prefix + " " + strings.Join(args, " ")
}

// Run 直接执行 socat 命令并阻塞到其结束。
func (o Options) Run(ctx context.Context) error {
	cmd, err := o.Command(ctx)
	if err != nil {
		return err
	}
	return cmd.Run()
}
