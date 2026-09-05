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
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/soulteary/portmap/internal/forward"
	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/proxy"
	"github.com/soulteary/portmap/internal/socat"
	"github.com/soulteary/portmap/internal/stats"
	"github.com/soulteary/portmap/internal/web"
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
	configPath  string
	lang        string
	statsAddr   string
	webAddr     string
	webLogMax   int
}

// String 返回最终生效配置的可读摘要，用于启动时打印，便于确认合并后的实际参数。
func (o options) String() string {
	listen := net.JoinHostPort(o.listenHost, strconv.Itoa(o.listenPort))
	return fmt.Sprintf(
		"mode=%s proto=%s listen=%s target=%s reuseaddr=%t dial-timeout=%s max-conns=%d idle-timeout=%s log-level=%s quiet=%t sudo=%t",
		o.mode, o.proto, listen, o.target, o.reuseAddr, o.dialTimeout, o.maxConns, o.idleTimeout, o.logLevel, o.quiet, o.useSudo,
	)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

// run 是根命令入口：先处理一次 -lang（供所有子命令与 --help 共享），
// 再解析子命令并分发。无子命令或第一个参数以 - 开头时默认走 forward，
// 以保持 `portmap -listen-port 22 -target ...` 的向后兼容。
func run(argv []string) error {
	// 语言在首次调用 T() 时按系统环境自动检测（见 i18n.Detect）。
	// 若命令行显式指定 -lang，则预扫描一次并覆盖，使 --help 与 flag
	// 描述也使用指定语言。preScanLang 会跳过子命令名。
	if code := preScanLang(argv); code != "" {
		if l, ok := i18n.ParseLang(code); ok {
			i18n.SetLang(l)
		}
	}

	sub, rest := splitSubcommand(argv)
	switch sub {
	case "forward":
		return runForward(rest)
	case "proxy":
		return runProxy(rest)
	case "version":
		fmt.Println(i18n.T(i18n.KeyVersionLine, version, commit, date))
		return nil
	default:
		return errors.New(i18n.T(i18n.KeyErrUnknownSub, sub))
	}
}

// subcommands 是所有已知子命令的集合。
var subcommands = map[string]bool{
	"forward": true,
	"proxy":   true,
	"version": true,
}

// splitSubcommand 从 argv 中解析子命令：
//   - 无参数，或第一个参数以 - 开头（即直接是 flag）时，默认 "forward" 并保留全部参数；
//   - 第一个参数为已知子命令时，返回该子命令与其余参数；
//   - 否则默认 "forward" 并保留全部参数（把未知位置参数交给 forward 解析报错）。
func splitSubcommand(argv []string) (string, []string) {
	if len(argv) == 0 {
		return "forward", argv
	}
	first := argv[0]
	if strings.HasPrefix(first, "-") {
		return "forward", argv
	}
	if subcommands[first] {
		return first, argv[1:]
	}
	return "forward", argv
}

// runForward 实现 forward 子命令：等价于既有的端口转发逻辑，flag 保持不变。
func runForward(argv []string) error {
	var opt options

	fs := flag.NewFlagSet("portmap", flag.ContinueOnError)
	fs.IntVar(&opt.listenPort, "listen-port", 22, i18n.T(i18n.KeyFlagListenPort))
	fs.StringVar(&opt.listenHost, "listen-host", "", i18n.T(i18n.KeyFlagListenHost))
	fs.StringVar(&opt.target, "target", "127.0.0.1:2222", i18n.T(i18n.KeyFlagTarget))
	fs.StringVar(&opt.mode, "mode", "go", i18n.T(i18n.KeyFlagMode))
	fs.StringVar(&opt.proto, "proto", "tcp", i18n.T(i18n.KeyFlagProto))
	fs.BoolVar(&opt.reuseAddr, "reuseaddr", true, i18n.T(i18n.KeyFlagReuseAddr))
	fs.BoolVar(&opt.useSudo, "sudo", false, i18n.T(i18n.KeyFlagSudo))
	fs.DurationVar(&opt.dialTimeout, "dial-timeout", 10*time.Second, i18n.T(i18n.KeyFlagDialTimeout))
	fs.IntVar(&opt.maxConns, "max-conns", 0, i18n.T(i18n.KeyFlagMaxConns))
	fs.DurationVar(&opt.idleTimeout, "idle-timeout", 0, i18n.T(i18n.KeyFlagIdleTimeout))
	fs.StringVar(&opt.logLevel, "log-level", "info", i18n.T(i18n.KeyFlagLogLevel))
	fs.BoolVar(&opt.quiet, "quiet", false, i18n.T(i18n.KeyFlagQuiet))
	fs.BoolVar(&opt.showVersion, "version", false, i18n.T(i18n.KeyFlagVersion))
	fs.StringVar(&opt.configPath, "config", "", i18n.T(i18n.KeyFlagConfig))
	fs.StringVar(&opt.lang, "lang", "", i18n.T(i18n.KeyFlagLang, strings.Join(i18n.Codes(), "/")))
	fs.StringVar(&opt.statsAddr, "stats-addr", "", i18n.T(i18n.KeyFlagStatsAddr))
	fs.StringVar(&opt.webAddr, "web-addr", "", i18n.T(i18n.KeyFlagWebAddr))
	fs.IntVar(&opt.webLogMax, "web-log-max", 1000, i18n.T(i18n.KeyFlagWebLogMax))

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T(i18n.KeyUsageTitle))
		fmt.Fprintln(os.Stderr, i18n.T(i18n.KeyUsageLine, "portmap"))
		fmt.Fprintf(os.Stderr, "\n%s\n\n", i18n.T(i18n.KeyUsageSubcommands))
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if len(argv) == 0 {
		fs.Usage()
		return nil
	}

	// -lang 已在 preScanLang 中生效；此处不再重复处理。

	if opt.showVersion {
		fmt.Println(i18n.T(i18n.KeyVersionLine, version, commit, date))
		return nil
	}

	// 记录用户显式设置过哪些 flag：既用于配置文件合并（flag 优先），
	// 也用于 socat 模式判断是否使用了仅 go 模式支持的选项。
	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// 多实例（多端口映射）：仅当 -config 指向的配置文件中 forward: 段声明了
	// 多个实例时触发；此时逐个实例转换为运行参数并发启动，per-instance 忽略
	// CLI 显式 flag（多实例场景下 -listen-port 等 flag 无法一一对应）。
	if opt.configPath != "" {
		topCfg, err := loadTopConfig(opt.configPath)
		if err != nil {
			return err
		}
		if len(topCfg.Forward) > 1 {
			return runForwardMulti(opt, topCfg, setFlags)
		}
	}

	// 加载配置文件并合并：仅对配置文件中出现、且命令行未显式设置的字段生效。
	// 优先级：命令行显式 flag > 配置文件 > 内置默认值。
	if opt.configPath != "" {
		cfg, err := loadForwardConfig(opt.configPath)
		if err != nil {
			return err
		}
		if err := mergeConfig(&opt, cfg, setFlags); err != nil {
			return err
		}
		// 若语言来自配置文件（命令行未显式 -lang，但配置文件提供了 lang），
		// 在此补一次 SetLang，使运行期消息（日志/错误）使用该语言。
		// 注意：--help 与 flag 描述在 flag 解析前已定稿，配置文件无法改变其语言。
		if !setFlags["lang"] && opt.lang != "" {
			if l, ok := i18n.ParseLang(opt.lang); ok {
				i18n.SetLang(l)
			}
		}
	}

	if opt.listenPort <= 0 || opt.listenPort > 65535 {
		return errors.New(i18n.T(i18n.KeyErrListenPort, opt.listenPort))
	}
	if strings.TrimSpace(opt.target) == "" {
		return errors.New(i18n.T(i18n.KeyErrTargetEmpty))
	}
	proto := strings.ToLower(strings.TrimSpace(opt.proto))
	if proto != "tcp" && proto != "udp" {
		return errors.New(i18n.T(i18n.KeyErrProto, opt.proto))
	}
	opt.proto = proto

	if opt.idleTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrIdleNeg, opt.idleTimeout))
	}
	if opt.maxConns < 0 {
		return errors.New(i18n.T(i18n.KeyErrMaxConnsNeg, opt.maxConns))
	}
	if opt.dialTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrDialNeg, opt.dialTimeout))
	}
	logLevel := strings.ToLower(strings.TrimSpace(opt.logLevel))
	if logLevel != "info" && logLevel != "debug" {
		return errors.New(i18n.T(i18n.KeyErrLogLevel, opt.logLevel))
	}
	opt.logLevel = logLevel

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch opt.mode {
	case "go":
		return runGo(ctx, opt)
	case "socat":
		return runSocat(ctx, opt, setFlags)
	default:
		return errors.New(i18n.T(i18n.KeyErrMode, opt.mode))
	}
}

// preScanLang 在正式解析 flag 前，从 argv 里粗略读取 -lang/--lang 的值，
// 以便 --help 与 flag 描述也能使用用户指定的语言。支持 "-lang zh" 与
// "-lang=zh" 两种写法。未找到时返回空串。若首个参数是子命令名（非 flag），
// 会被跳过，从而支持 "portmap proxy -lang zh" 的写法。
func preScanLang(argv []string) string {
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") && subcommands[argv[0]] {
		argv = argv[1:]
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			break
		}
		name := strings.TrimLeft(a, "-")
		if name == "lang" {
			if i+1 < len(argv) {
				return argv[i+1]
			}
			return ""
		}
		if v, ok := strings.CutPrefix(name, "lang="); ok {
			return v
		}
	}
	return ""
}

func runGo(ctx context.Context, opt options) error {
	log.Print(i18n.T(i18n.KeyLogEffectiveConfig, opt))
	listen := net.JoinHostPort(opt.listenHost, strconv.Itoa(opt.listenPort))
	// 仅当启用 Web 面板（-web-addr 非空）时创建事件日志并注入 forward，
	// 未启用时保持 nil，避免无谓的内存与插桩开销。
	var events *stats.EventLog
	if strings.TrimSpace(opt.webAddr) != "" {
		events = stats.NewEventLog(opt.webLogMax)
	}
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
		Events:      events,
	})
	watchStatusSignal(ctx, srv)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// forward 无 allow-public 概念，统计端点默认仅回环。
	// 统计端点启动失败（如非回环拒绝）会 cancel ctx，从而中止 forward。
	statsWait, statsErrCh := startStatsEndpoint(ctx, cancel, opt.statsAddr, false, srv)
	defer statsWait()
	// Web 面板同样默认仅回环（allowPublic=false），与 stats-addr 行为一致。
	webWait, webErrCh := startWebEndpoint(ctx, cancel, opt.webAddr, false, events, srv)
	defer webWait()

	serveErr := srv.ListenAndServe(ctx)
	cancel()
	statsWait()
	webWait()
	statsErr := drainStatsErr(statsErrCh)
	webErr := drainStatsErr(webErrCh)
	if serveErr != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrServeExit), serveErr)
	}
	if statsErr != nil {
		return statsErr
	}
	if webErr != nil {
		return webErr
	}
	return nil
}

// drainStatsErr 在统计端点 goroutine 已退出后，非阻塞地取出可能存在的错误。
func drainStatsErr(ch <-chan error) error {
	select {
	case err, ok := <-ch:
		if ok {
			return err
		}
	default:
	}
	return nil
}

func runSocat(ctx context.Context, opt options, setFlags map[string]bool) error {
	log.Print(i18n.T(i18n.KeyLogEffectiveConfig, opt))
	// socat 模式仅复用系统 socat，以下仅 go 模式支持的 flag 若被显式设置则提示忽略。
	goOnly := []string{"idle-timeout", "max-conns", "log-level", "quiet"}
	var ignored []string
	for _, name := range goOnly {
		if setFlags[name] {
			ignored = append(ignored, "-"+name)
		}
	}
	if len(ignored) > 0 {
		log.Print(i18n.T(i18n.KeyLogSocatIgnore, strings.Join(ignored, " ")))
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
	log.Print(i18n.T(i18n.KeyLogSocatExec, socatOpt.String()))
	if err := socatOpt.Run(ctx); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrSocatFailed), err)
	}
	return nil
}

// forwardPerInstanceFlags 是「per-instance」语义的 forward flag（对应单个实例的
// 监听/转发行为）。多实例场景下这些 flag 无法一一对应到某个实例，若被显式设置
// 则提示忽略。-config/-lang/-version 等非 per-instance flag 不在此列。
var forwardPerInstanceFlags = []string{
	"listen-port", "listen-host", "target", "mode", "proto",
	"reuseaddr", "sudo", "dial-timeout", "max-conns", "idle-timeout",
	"log-level", "quiet",
}

// normalizeForwardOptions 归一化并校验单个 forward 实例的运行参数。
// 复用与单实例路径一致的校验规则（端口范围、target 非空、proto、超时非负、
// log-level 等），并就地归一化 proto/log-level 的大小写。
func normalizeForwardOptions(opt *options) error {
	if opt.listenPort <= 0 || opt.listenPort > 65535 {
		return errors.New(i18n.T(i18n.KeyErrListenPort, opt.listenPort))
	}
	if strings.TrimSpace(opt.target) == "" {
		return errors.New(i18n.T(i18n.KeyErrTargetEmpty))
	}
	proto := strings.ToLower(strings.TrimSpace(opt.proto))
	if proto != "tcp" && proto != "udp" {
		return errors.New(i18n.T(i18n.KeyErrProto, opt.proto))
	}
	opt.proto = proto
	if opt.idleTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrIdleNeg, opt.idleTimeout))
	}
	if opt.maxConns < 0 {
		return errors.New(i18n.T(i18n.KeyErrMaxConnsNeg, opt.maxConns))
	}
	if opt.dialTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrDialNeg, opt.dialTimeout))
	}
	logLevel := strings.ToLower(strings.TrimSpace(opt.logLevel))
	if logLevel != "info" && logLevel != "debug" {
		return errors.New(i18n.T(i18n.KeyErrLogLevel, opt.logLevel))
	}
	opt.logLevel = logLevel
	return nil
}

// forwardListenAddr 返回 forward 实例的规范化监听地址，用于重复检测与日志。
func forwardListenAddr(opt options) string {
	return net.JoinHostPort(opt.listenHost, strconv.Itoa(opt.listenPort))
}

// defaultForwardOptions 返回与 forward flag 默认值一致的基线运行参数，供多实例
// 逐个转换时作为起点（每个实例从默认值出发，再叠加配置文件字段）。
func defaultForwardOptions() options {
	return options{
		listenPort:  22,
		target:      "127.0.0.1:2222",
		mode:        "go",
		proto:       "tcp",
		reuseAddr:   true,
		dialTimeout: 10 * time.Second,
		maxConns:    0,
		idleTimeout: 0,
		logLevel:    "info",
		webLogMax:   1000,
	}
}

// runForwardMulti 处理 forward 多实例（多端口映射）：逐个实例从默认值出发叠加
// 配置文件字段（per-instance 不应用 CLI 覆盖），校验并去重后并发启动。
func runForwardMulti(base options, cfg *Config, setFlags map[string]bool) error {
	// 语言：配置文件的 lang 在命令行未显式设置时生效（与单实例路径一致）。
	if cfg.Lang != nil && !setFlags["lang"] {
		if l, ok := i18n.ParseLang(*cfg.Lang); ok {
			i18n.SetLang(l)
		}
	}

	// 提示：多实例场景忽略 per-instance CLI flag（如 -listen-port）。
	var ignored []string
	for _, name := range forwardPerInstanceFlags {
		if setFlags[name] {
			ignored = append(ignored, "-"+name)
		}
	}
	if len(ignored) > 0 {
		log.Print(i18n.T(i18n.KeyLogMultiIgnoreFlags, strings.Join(ignored, " ")))
	}

	instances := make([]options, 0, len(cfg.Forward))
	seen := make(map[string]struct{}, len(cfg.Forward))
	for _, fc := range cfg.Forward {
		opt := defaultForwardOptions()
		opt.lang = base.lang
		// per-instance 不应用 CLI 覆盖：setFlags 传空 map。
		if err := applyForwardConfig(&opt, fc, map[string]bool{}); err != nil {
			return err
		}
		if err := normalizeForwardOptions(&opt); err != nil {
			return err
		}
		addr := forwardListenAddr(opt)
		if _, dup := seen[addr]; dup {
			return errors.New(i18n.T(i18n.KeyErrDuplicateListen, addr))
		}
		seen[addr] = struct{}{}
		instances = append(instances, opt)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return startForwardInstances(ctx, instances)
}

// startForwardInstances 并发启动多个 forward 实例（均为 go 模式），聚合信号与
// 错误：任一实例致命错误即返回；ctx 取消（收到退出信号）时各实例经其自身的
// ListenAndServe(ctx) 优雅关闭。单实例场景亦可复用（列表长度 1）。
func startForwardInstances(ctx context.Context, instances []options) error {
	log.Print(i18n.T(i18n.KeyLogForwardStartingInstances, len(instances)))

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(instances))
	var wg sync.WaitGroup
	providers := make([]stats.Provider, 0, len(instances))
	statsAddr := ""
	// Web 面板：若任一实例设置了 -web-addr，则创建一份共享事件日志并注入所有
	// 实例，使多实例聚合到同一个面板。取第一个非空 webAddr 与其 webLogMax。
	webAddr := ""
	webLogMax := 1000
	for _, opt := range instances {
		if strings.TrimSpace(opt.webAddr) != "" {
			webAddr = opt.webAddr
			webLogMax = opt.webLogMax
			break
		}
	}
	var events *stats.EventLog
	if webAddr != "" {
		events = stats.NewEventLog(webLogMax)
	}
	for i, opt := range instances {
		i, opt := i, opt
		listen := forwardListenAddr(opt)
		log.Print(i18n.T(i18n.KeyLogInstanceStarting, i+1, opt.String()))
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
			Events:      events,
		})
		providers = append(providers, srv)
		if statsAddr == "" && strings.TrimSpace(opt.statsAddr) != "" {
			statsAddr = opt.statsAddr
		}
		watchStatusSignal(ctx, srv)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := srv.ListenAndServe(ctx); err != nil {
				// ctx 取消导致的关闭视为正常退出，不作为致命错误。
				if ctx.Err() == nil {
					errCh <- fmt.Errorf(i18n.T(i18n.KeyErrInstanceFailed), listen, err)
					cancel()
				}
			}
		}()
	}

	statsWait, statsErrCh := startStatsEndpoint(ctx, cancel, statsAddr, false, providers...)
	webWait, webErrCh := startWebEndpoint(ctx, cancel, webAddr, false, events, providers...)

	wg.Wait()
	statsWait()
	webWait()
	close(errCh)
	if err := <-errCh; err != nil {
		return err
	}
	if statsErr := drainStatsErr(statsErrCh); statsErr != nil {
		return statsErr
	}
	if webErr := drainStatsErr(webErrCh); webErr != nil {
		return webErr
	}
	return nil
}

// proxyOptions 保存 proxy 子命令的运行参数。
type proxyOptions struct {
	addr                         string
	dialTimeout                  time.Duration
	maxConns                     int
	handshakeTimeout             time.Duration
	idleTimeout                  time.Duration
	allowPublic                  bool
	upstream                     string
	upstreamIdentity             string
	upstreamIdentityPassphrase   string
	upstreamKnownHosts           string
	upstreamInsecure             bool
	upstreamKeepalive            time.Duration
	upstreamKeepaliveMaxFailures int
	showVersion                  bool
	configPath                   string
	lang                         string
	statsAddr                    string
	statsAllowPublic             bool
	webAddr                      string
	webAllowPublic               bool
	webLogMax                    int
}

// runProxy 实现 proxy 子命令：单端口 SOCKS5 + HTTP 应用层代理。
func runProxy(argv []string) error {
	var opt proxyOptions

	fs := flag.NewFlagSet("portmap proxy", flag.ContinueOnError)
	fs.StringVar(&opt.addr, "addr", "127.0.0.1:1080", i18n.T(i18n.KeyFlagProxyAddr))
	fs.DurationVar(&opt.dialTimeout, "dial-timeout", 30*time.Second, i18n.T(i18n.KeyFlagProxyDialTimeout))
	fs.IntVar(&opt.maxConns, "max-conns", 256, i18n.T(i18n.KeyFlagProxyMaxConns))
	fs.DurationVar(&opt.handshakeTimeout, "handshake-timeout", 10*time.Second, i18n.T(i18n.KeyFlagProxyHandshakeTimeout))
	fs.DurationVar(&opt.idleTimeout, "idle-timeout", 5*time.Minute, i18n.T(i18n.KeyFlagProxyIdleTimeout))
	fs.BoolVar(&opt.allowPublic, "allow-public", false, i18n.T(i18n.KeyFlagProxyAllowPublic))
	fs.StringVar(&opt.upstream, "upstream", "", i18n.T(i18n.KeyFlagProxyUpstream))
	fs.StringVar(&opt.upstreamIdentity, "upstream-identity", "", i18n.T(i18n.KeyFlagProxyUpstreamIdentity))
	fs.StringVar(&opt.upstreamKnownHosts, "upstream-known-hosts", "", i18n.T(i18n.KeyFlagProxyUpstreamKnownHosts))
	fs.BoolVar(&opt.upstreamInsecure, "upstream-insecure", false, i18n.T(i18n.KeyFlagProxyUpstreamInsecure))
	fs.DurationVar(&opt.upstreamKeepalive, "upstream-keepalive", 0, i18n.T(i18n.KeyFlagProxyUpstreamKeepalive))
	fs.IntVar(&opt.upstreamKeepaliveMaxFailures, "upstream-keepalive-max-failures", 0, i18n.T(i18n.KeyFlagProxyUpstreamKeepaliveMaxFailures))
	fs.BoolVar(&opt.showVersion, "version", false, i18n.T(i18n.KeyFlagVersion))
	fs.StringVar(&opt.configPath, "config", "", i18n.T(i18n.KeyFlagConfig))
	fs.StringVar(&opt.lang, "lang", "", i18n.T(i18n.KeyFlagLang, strings.Join(i18n.Codes(), "/")))
	fs.StringVar(&opt.statsAddr, "stats-addr", "", i18n.T(i18n.KeyFlagStatsAddr))
	fs.BoolVar(&opt.statsAllowPublic, "stats-allow-public", false, i18n.T(i18n.KeyFlagStatsAllowPublic))
	fs.StringVar(&opt.webAddr, "web-addr", "", i18n.T(i18n.KeyFlagWebAddr))
	fs.BoolVar(&opt.webAllowPublic, "web-allow-public", false, i18n.T(i18n.KeyFlagWebAllowPublic))
	fs.IntVar(&opt.webLogMax, "web-log-max", 1000, i18n.T(i18n.KeyFlagWebLogMax))

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\n", i18n.T(i18n.KeyProxyUsageTitle))
		fmt.Fprintln(os.Stderr, i18n.T(i18n.KeyProxyUsageLine, "portmap"))
		fs.PrintDefaults()
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if opt.showVersion {
		fmt.Println(i18n.T(i18n.KeyVersionLine, version, commit, date))
		return nil
	}

	setFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// 多实例（多端口映射）：仅当 -config 指向的配置文件中 proxy: 段声明了多个
	// 实例时触发；逐个实例转换为运行参数并发启动，per-instance 忽略 CLI 显式 flag。
	if opt.configPath != "" {
		topCfg, err := loadTopConfig(opt.configPath)
		if err != nil {
			return err
		}
		if len(topCfg.Proxy) > 1 {
			return runProxyMulti(opt, topCfg, setFlags)
		}
	}

	// 加载配置文件并合并 proxy 段：命令行显式 flag > 配置文件 > 默认值。
	if opt.configPath != "" {
		cfg, err := loadTopConfig(opt.configPath)
		if err != nil {
			return err
		}
		if err := mergeProxyConfig(&opt, cfg, setFlags); err != nil {
			return err
		}
		if !setFlags["lang"] && opt.lang != "" {
			if l, ok := i18n.ParseLang(opt.lang); ok {
				i18n.SetLang(l)
			}
		}
	}

	if err := validateProxyOptions(&opt); err != nil {
		return err
	}

	// 解析上游代理配置（留空表示直连，保持向后兼容）。
	resolveIdentityPassphrase(&opt)
	upstream, err := buildProxyUpstream(&opt)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := newProxyServer(opt, upstream)
	// 仅当启用 Web 面板时创建事件日志并注入 proxy 服务。
	var events *stats.EventLog
	if strings.TrimSpace(opt.webAddr) != "" {
		events = stats.NewEventLog(opt.webLogMax)
		srv.Events = events
	}
	return serveProxy(ctx, srv, opt.statsAddr, opt.statsAllowPublic, opt.webAddr, opt.webAllowPublic, events)
}

// validateProxyOptions 校验单个 proxy 实例的运行参数（地址非空、各超时非负、
// max-conns 非负），与单实例路径规则一致。
func validateProxyOptions(opt *proxyOptions) error {
	if strings.TrimSpace(opt.addr) == "" {
		return errors.New(i18n.T(i18n.KeyErrTargetEmpty))
	}
	if opt.dialTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrDialNeg, opt.dialTimeout))
	}
	if opt.maxConns < 0 {
		return errors.New(i18n.T(i18n.KeyErrMaxConnsNeg, opt.maxConns))
	}
	if opt.handshakeTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrProxyHandshakeNeg, opt.handshakeTimeout))
	}
	if opt.idleTimeout < 0 {
		return errors.New(i18n.T(i18n.KeyErrIdleNeg, opt.idleTimeout))
	}
	return nil
}

// resolveIdentityPassphrase 按优先级填充 opt.upstreamIdentityPassphrase：
//
//	环境变量 PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE > 已有值（来自 YAML 配置）
//	> 交互式终端输入。
//
// 交互式输入仅在以下条件同时满足时触发：环境变量与已有值均为空、配置了
// -upstream-identity 私钥文件、且 stdin 是 TTY。读取使用 term.ReadPassword，
// 不回显。passphrase 不会写入日志。出于安全考虑不提供命令行 flag，避免明文
// 出现在进程列表或 shell 历史中。
func resolveIdentityPassphrase(opt *proxyOptions) {
	if env := os.Getenv("PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE"); env != "" {
		opt.upstreamIdentityPassphrase = env
		return
	}
	if opt.upstreamIdentityPassphrase != "" {
		return
	}
	// 仅当配置了私钥文件且 stdin 是交互式终端时才提示输入。
	if strings.TrimSpace(opt.upstreamIdentity) == "" {
		return
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}
	fmt.Fprint(os.Stderr, i18n.T(i18n.KeyPromptUpstreamIdentityPassphrase, opt.addr))
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return
	}
	opt.upstreamIdentityPassphrase = string(pw)
}

// buildProxyUpstream 依据 proxyOptions 解析上游代理链配置；upstream 为空表示
// 直连（返回 nil, nil），保持向后兼容。
func buildProxyUpstream(opt *proxyOptions) (*proxy.UpstreamConfig, error) {
	if strings.TrimSpace(opt.upstream) == "" {
		return nil, nil
	}
	u, err := proxy.ParseUpstreamURL(opt.upstream)
	if err != nil {
		return nil, err
	}
	u.IdentityFile = opt.upstreamIdentity
	u.IdentityPassphrase = opt.upstreamIdentityPassphrase
	u.KnownHostsFile = opt.upstreamKnownHosts
	u.Insecure = opt.upstreamInsecure
	u.KeepaliveInterval = opt.upstreamKeepalive
	u.KeepaliveMaxFailures = opt.upstreamKeepaliveMaxFailures
	return u, nil
}

// newProxyServer 依据运行参数与上游配置构造 proxy.Server。
func newProxyServer(opt proxyOptions, upstream *proxy.UpstreamConfig) *proxy.Server {
	srv := proxy.New(opt.addr)
	srv.DialTimeout = opt.dialTimeout
	srv.MaxConns = opt.maxConns
	srv.HandshakeTimeout = opt.handshakeTimeout
	srv.IdleTimeout = opt.idleTimeout
	srv.AllowPublic = opt.allowPublic
	srv.Upstream = upstream
	return srv
}

// startStatsEndpoint 在 addr 非空时并发启动统计 HTTP 端点（/stats、/metrics），
// 聚合 providers 的快照。ctx 取消时优雅关闭。启动/运行出错时调用 onErr（通常为
// ctx 的 cancel），使主服务随之退出，并把错误送入返回的通道供调用方读取。
// addr 为空时返回的 wait 为 no-op、错误通道立即关闭。allowPublic 控制是否允许
// 非回环监听。
func startStatsEndpoint(ctx context.Context, onErr context.CancelFunc, addr string, allowPublic bool, providers ...stats.Provider) (wait func(), errCh <-chan error) {
	ch := make(chan error, 1)
	if strings.TrimSpace(addr) == "" {
		close(ch)
		return func() {}, ch
	}
	srv := &stats.Server{
		Addr:        addr,
		AllowPublic: allowPublic,
		Providers:   providers,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ListenAndServe(ctx); err != nil {
			ch <- err
			if onErr != nil {
				onErr()
			}
		}
	}()
	return func() { <-done }, ch
}

// startWebEndpoint 在 addr 非空时并发启动 Web 面板 HTTP 端点（/、/api/stats、
// /api/logs），聚合 providers 的快照并展示 events 中的连接事件。ctx 取消时优雅
// 关闭。启动/运行出错时调用 onErr（通常为 ctx 的 cancel），使主服务随之退出，
// 并把错误送入返回的通道供调用方读取。addr 为空时返回的 wait 为 no-op、错误通道
// 立即关闭。allowPublic 控制是否允许非回环监听。
func startWebEndpoint(ctx context.Context, onErr context.CancelFunc, addr string, allowPublic bool, events *stats.EventLog, providers ...stats.Provider) (wait func(), errCh <-chan error) {
	ch := make(chan error, 1)
	if strings.TrimSpace(addr) == "" {
		close(ch)
		return func() {}, ch
	}
	srv := &web.Server{
		Addr:        addr,
		AllowPublic: allowPublic,
		Providers:   providers,
		Events:      events,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ListenAndServe(ctx); err != nil {
			ch <- err
			if onErr != nil {
				onErr()
			}
		}
	}()
	return func() { <-done }, ch
}

// serveProxy 启动单个 proxy.Server 并处理优雅关闭：ctx 取消时对其 Shutdown。
// 若 statsAddr 非空则并发启动统计 HTTP 端点，聚合该 proxy 的快照；若 webAddr
// 非空则并发启动 Web 面板端点，聚合该 proxy 的快照与其 events 事件日志。
func serveProxy(ctx context.Context, srv *proxy.Server, statsAddr string, statsAllowPublic bool, webAddr string, webAllowPublic bool, events *stats.EventLog) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	statsWait, statsErrCh := startStatsEndpoint(ctx, cancel, statsAddr, statsAllowPublic, srv)
	defer statsWait()

	webWait, webErrCh := startWebEndpoint(ctx, cancel, webAddr, webAllowPublic, events, srv)
	defer webWait()

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		log.Println(i18n.T(i18n.KeyLogProxyShuttingDown))
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf(i18n.T(i18n.KeyLogProxyShutdownFailed), err)
		}
	}()

	err := srv.ListenAndServe()
	cancel()
	if ctx.Err() != nil {
		<-shutdownDone
	}
	statsWait()
	webWait()
	statsErr := drainStatsErr(statsErrCh)
	webErr := drainStatsErr(webErrCh)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyExit), err)
	}
	if statsErr != nil {
		return statsErr
	}
	if webErr != nil {
		return webErr
	}
	return nil
}

// proxyPerInstanceFlags 是「per-instance」语义的 proxy flag，多实例场景下无法
// 一一对应，若被显式设置则提示忽略。-config/-lang/-version 不在此列。
var proxyPerInstanceFlags = []string{
	"addr", "dial-timeout", "max-conns", "handshake-timeout", "idle-timeout",
	"allow-public", "upstream", "upstream-identity", "upstream-known-hosts",
	"upstream-insecure", "upstream-keepalive", "upstream-keepalive-max-failures",
}

// defaultProxyOptions 返回与 proxy flag 默认值一致的基线运行参数，供多实例逐个
// 转换时作为起点。
func defaultProxyOptions() proxyOptions {
	return proxyOptions{
		addr:             "127.0.0.1:1080",
		dialTimeout:      30 * time.Second,
		maxConns:         256,
		handshakeTimeout: 10 * time.Second,
		idleTimeout:      5 * time.Minute,
		webLogMax:        1000,
	}
}

// runProxyMulti 处理 proxy 多实例（多端口映射）：逐个实例从默认值出发叠加配置
// 文件字段（per-instance 不应用 CLI 覆盖），校验并按监听地址去重后并发启动。
func runProxyMulti(base proxyOptions, cfg *Config, setFlags map[string]bool) error {
	if cfg.Lang != nil && !setFlags["lang"] {
		if l, ok := i18n.ParseLang(*cfg.Lang); ok {
			i18n.SetLang(l)
		}
	}

	var ignored []string
	for _, name := range proxyPerInstanceFlags {
		if setFlags[name] {
			ignored = append(ignored, "-"+name)
		}
	}
	if len(ignored) > 0 {
		log.Print(i18n.T(i18n.KeyLogMultiIgnoreFlags, strings.Join(ignored, " ")))
	}

	type proxyInstance struct {
		opt      proxyOptions
		upstream *proxy.UpstreamConfig
	}
	instances := make([]proxyInstance, 0, len(cfg.Proxy))
	seen := make(map[string]struct{}, len(cfg.Proxy))
	for _, pc := range cfg.Proxy {
		opt := defaultProxyOptions()
		opt.lang = base.lang
		if err := applyProxyConfig(&opt, pc, map[string]bool{}); err != nil {
			return err
		}
		if setFlags["stats-addr"] {
			opt.statsAddr = base.statsAddr
		}
		if setFlags["stats-allow-public"] {
			opt.statsAllowPublic = base.statsAllowPublic
		}
		if setFlags["web-addr"] {
			opt.webAddr = base.webAddr
		}
		if setFlags["web-allow-public"] {
			opt.webAllowPublic = base.webAllowPublic
		}
		if setFlags["web-log-max"] {
			opt.webLogMax = base.webLogMax
		}
		if err := validateProxyOptions(&opt); err != nil {
			return err
		}
		if _, dup := seen[opt.addr]; dup {
			return errors.New(i18n.T(i18n.KeyErrDuplicateListen, opt.addr))
		}
		seen[opt.addr] = struct{}{}
		resolveIdentityPassphrase(&opt)
		upstream, err := buildProxyUpstream(&opt)
		if err != nil {
			return err
		}
		instances = append(instances, proxyInstance{opt: opt, upstream: upstream})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Print(i18n.T(i18n.KeyLogProxyStartingInstances, len(instances)))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(instances))
	var wg sync.WaitGroup
	providers := make([]stats.Provider, 0, len(instances))
	statsAddr := ""
	statsAllowPublic := false
	// Web 面板：若任一实例设置了 -web-addr，则创建一份共享事件日志并注入所有
	// 实例，聚合到同一个面板。取第一个非空 webAddr 及其 webLogMax/allowPublic。
	webAddr := ""
	webAllowPublic := false
	webLogMax := 1000
	for _, inst := range instances {
		if strings.TrimSpace(inst.opt.webAddr) != "" {
			webAddr = inst.opt.webAddr
			webAllowPublic = inst.opt.webAllowPublic
			webLogMax = inst.opt.webLogMax
			break
		}
	}
	var events *stats.EventLog
	if webAddr != "" {
		events = stats.NewEventLog(webLogMax)
	}
	for i, inst := range instances {
		i, inst := i, inst
		log.Print(i18n.T(i18n.KeyLogInstanceStarting, i+1, inst.opt.addr))
		srv := newProxyServer(inst.opt, inst.upstream)
		srv.Events = events
		providers = append(providers, srv)
		if statsAddr == "" && strings.TrimSpace(inst.opt.statsAddr) != "" {
			statsAddr = inst.opt.statsAddr
			statsAllowPublic = inst.opt.statsAllowPublic
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// per-instance 禁用 stats/web 端点，改为循环后聚合启动一份。
			if err := serveProxy(ctx, srv, "", false, "", false, nil); err != nil {
				if ctx.Err() == nil {
					errCh <- fmt.Errorf(i18n.T(i18n.KeyErrInstanceFailed), inst.opt.addr, err)
					cancel()
				}
			}
		}()
	}

	statsWait, statsErrCh := startStatsEndpoint(ctx, cancel, statsAddr, statsAllowPublic, providers...)
	webWait, webErrCh := startWebEndpoint(ctx, cancel, webAddr, webAllowPublic, events, providers...)

	wg.Wait()
	statsWait()
	webWait()
	close(errCh)
	if err := <-errCh; err != nil {
		return err
	}
	if statsErr := drainStatsErr(statsErrCh); statsErr != nil {
		return statsErr
	}
	if webErr := drainStatsErr(webErrCh); webErr != nil {
		return webErr
	}
	return nil
}
