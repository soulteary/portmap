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

// Command portmap 实现等价于
//
//	sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222
//
// 的端口转发。默认使用纯 Go 实现，也可通过 -mode socat 直接调用系统 socat。
// 支持 TCP/UDP、并发限流、空闲超时与连接级日志。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/soulteary/portmap/internal/forward"
	"github.com/soulteary/portmap/internal/socat"
)

// 以下变量通过 -ldflags "-X main.version=... -X main.commit=... -X main.date=..." 注入。
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type options struct {
	listenPort  int
	listenHost  string
	target      string
	mode        string
	proto       string
	reuseAddr   bool
	useSudo     bool
	dialTimeout time.Duration
	maxConns    int
	idleTimeout time.Duration
	logLevel    string
	quiet       bool
	showVersion bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(argv []string) error {
	var opt options

	fs := flag.NewFlagSet("portmap", flag.ContinueOnError)
	fs.IntVar(&opt.listenPort, "listen-port", 22, "本地监听端口")
	fs.StringVar(&opt.listenHost, "listen-host", "", "本地监听地址（默认所有网卡）")
	fs.StringVar(&opt.target, "target", "127.0.0.1:2222", "转发目标地址 host:port")
	fs.StringVar(&opt.mode, "mode", "go", "转发模式：go（纯 Go 实现）或 socat（调用系统 socat）")
	fs.StringVar(&opt.proto, "proto", "tcp", "转发协议：tcp 或 udp")
	fs.BoolVar(&opt.reuseAddr, "reuseaddr", true, "启用 SO_REUSEADDR")
	fs.BoolVar(&opt.useSudo, "sudo", false, "socat 模式下是否以 sudo 运行")
	fs.DurationVar(&opt.dialTimeout, "dial-timeout", 10*time.Second, "拨号到目标的超时时间")
	fs.IntVar(&opt.maxConns, "max-conns", 0, "最大并发连接数，0 表示不限制（仅 go 模式；UDP 下限制并发会话数）")
	fs.DurationVar(&opt.idleTimeout, "idle-timeout", 0, "空闲超时，双向无数据则断开，0 表示不启用（仅 go 模式；UDP 下 0 表示默认 60s 回收空闲会话）")
	fs.StringVar(&opt.logLevel, "log-level", "info", "日志级别：info 或 debug（仅 go 模式）")
	fs.BoolVar(&opt.quiet, "quiet", false, "安静模式，抑制每连接的常规日志（仅 go 模式）")
	fs.BoolVar(&opt.showVersion, "version", false, "打印版本信息后退出")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "portmap - TCP/UDP 端口转发 (socat 等价实现)\n\n")
		fmt.Fprintf(os.Stderr, "用法: %s [flags]\n\n等价于: sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags:\n", "portmap")
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}

	if opt.showVersion {
		fmt.Printf("portmap %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	if opt.listenPort <= 0 || opt.listenPort > 65535 {
		return fmt.Errorf("非法监听端口: %d", opt.listenPort)
	}
	if strings.TrimSpace(opt.target) == "" {
		return fmt.Errorf("target 不能为空")
	}
	proto := strings.ToLower(strings.TrimSpace(opt.proto))
	if proto != "tcp" && proto != "udp" {
		return fmt.Errorf("未知 proto: %q（可选 tcp 或 udp）", opt.proto)
	}
	opt.proto = proto

	if opt.idleTimeout < 0 {
		return fmt.Errorf("idle-timeout 不能为负: %s", opt.idleTimeout)
	}
	if opt.maxConns < 0 {
		return fmt.Errorf("max-conns 不能为负: %d", opt.maxConns)
	}
	if opt.dialTimeout < 0 {
		return fmt.Errorf("dial-timeout 不能为负: %s", opt.dialTimeout)
	}
	logLevel := strings.ToLower(strings.TrimSpace(opt.logLevel))
	if logLevel != "info" && logLevel != "debug" {
		return fmt.Errorf("未知 log-level: %q（可选 info 或 debug）", opt.logLevel)
	}
	opt.logLevel = logLevel

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 记录用户显式设置过哪些 flag，供 socat 模式判断是否使用了仅 go 模式支持的选项。
	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	switch opt.mode {
	case "go":
		return runGo(ctx, opt)
	case "socat":
		return runSocat(ctx, opt, setFlags)
	default:
		return fmt.Errorf("未知 mode: %q（可选 go 或 socat）", opt.mode)
	}
}

func runGo(ctx context.Context, opt options) error {
	listen := net.JoinHostPort(opt.listenHost, strconv.Itoa(opt.listenPort))
	srv := forward.New(forward.Config{
		Listen:      listen,
		Target:      opt.target,
		Network:     opt.proto,
		ReuseAddr:   opt.reuseAddr,
		DialTimeout: opt.dialTimeout,
		MaxConns:    opt.maxConns,
		IdleTimeout: opt.idleTimeout,
		Debug:       strings.EqualFold(opt.logLevel, "debug"),
		Quiet:       opt.quiet,
	})
	watchStatusSignal(ctx, srv)
	if err := srv.ListenAndServe(ctx); err != nil {
		return fmt.Errorf("转发服务退出: %w", err)
	}
	return nil
}

func runSocat(ctx context.Context, opt options, setFlags map[string]bool) error {
	// socat 模式仅复用系统 socat，以下仅 go 模式支持的 flag 若被显式设置则提示忽略。
	goOnly := []string{"idle-timeout", "max-conns", "log-level", "quiet"}
	var ignored []string
	for _, name := range goOnly {
		if setFlags[name] {
			ignored = append(ignored, "-"+name)
		}
	}
	if len(ignored) > 0 {
		log.Printf("提示: socat 模式忽略以下仅 go 模式支持的参数: %s", strings.Join(ignored, " "))
	}

	socatOpt := socat.Options{
		ListenPort: opt.listenPort,
		Target:     opt.target,
		Proto:      opt.proto,
		Fork:       true,
		ReuseAddr:  opt.reuseAddr,
		ListenHost: opt.listenHost,
		Sudo:       opt.useSudo,
	}
	log.Printf("执行: %s", socatOpt.String())
	if err := socatOpt.Run(ctx); err != nil {
		return fmt.Errorf("socat 执行失败: %w", err)
	}
	return nil
}
