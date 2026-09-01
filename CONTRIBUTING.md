# Contributing to portmap | 贡献指南

<p align="center">
  <a href="#contributing-english">ENGLISH</a> | <a href="#贡献指南中文">中文</a>
</p>

Thanks for taking the time to contribute! 感谢你抽出时间参与贡献！

Before you start, please read our [Code of Conduct](CODE_OF_CONDUCT.md).
在开始之前，请先阅读我们的[行为准则](CODE_OF_CONDUCT.md)。

---

## Contributing (English)

### Ways to Contribute

- **Report bugs** — open an [issue](https://github.com/soulteary/portmap/issues)
  with a clear description and reproduction steps.
- **Suggest features** — open an issue describing the use case and motivation.
- **Improve docs** — fix typos, clarify wording, or translate. Docs are bilingual
  (`README.md` / `README.en.md`); please keep both in sync when relevant.
- **Submit code** — fix a bug or implement a feature via a pull request.

### Prerequisites

- **Go 1.27+** (the exact version is pinned in [`go.mod`](go.mod)).
- **make** — used to drive common build/test/lint tasks.
- **golangci-lint** `v2.13.2` — matches the version used in CI
  (see [`.github/workflows/ci.yml`](.github/workflows/ci.yml)).
- **socat** *(optional)* — only needed to exercise the `socat` mode locally.
- **goreleaser** *(optional)* — only needed to dry-run the release flow.

### Getting Started

```bash
# 1. Fork on GitHub, then clone your fork
git clone https://github.com/<your-username>/portmap.git
cd portmap

# 2. Add the upstream remote to keep in sync
git remote add upstream https://github.com/soulteary/portmap.git

# 3. Build and run the test suite
make build
make test
```

### Development Workflow

1. Create a topic branch from `main`:

```bash
git switch -c fix/short-description
```

2. Make your change. Keep commits focused and self-contained.
3. Run the full local checks (see below) — they mirror CI.
4. Push and open a pull request against `main`.

### Local Checks

Run these before opening a PR — they mirror what CI enforces:

```bash
make vet    # go vet ./...
make test   # go test ./... -race -count=1
make lint   # golangci-lint run ./...
make build  # build with version info injected
make security # govulncheck
```

CI runs `go vet` and `go test -race` on Linux, macOS, and Windows, plus separate
lint, module-hygiene, and vulnerability jobs. It cross-compiles every release
target, including `windows/arm64`, so please keep the code portable across platforms.

### Coding Guidelines

- **Formatting** — run `gofmt` / `goimports`; keep code idiomatic Go.
- **Linting** — code must pass `golangci-lint` (config in
  [`.golangci.yml`](.golangci.yml)).
- **Tests** — add or update tests for any behavior change. New packages should
  ship with `_test.go` coverage. Existing tests must keep passing under `-race`.
- **Cross-platform** — this project targets Unix-like systems and Windows.
  Platform-specific code uses build tags and split files (e.g.
  `signals_unix.go` / `signals_windows.go`,
  `reuseaddr_unix.go` / `reuseaddr_windows.go`). Follow this pattern rather than
  runtime OS checks where possible.
- **Comments** — explain intent and trade-offs, not the obvious.

### Internationalization (i18n)

User-facing help text, logs, and error messages are localized in
`internal/i18n` (`en`/`zh`/`ja`/`ko`/`fr`/`de`). When you add or change such a
message:

1. Add a message key constant in `internal/i18n/keys.go`.
2. Provide translations in **every** `internal/i18n/messages_*.go` file. If you
   cannot translate a language, add the English string as a placeholder and note
   it in the PR so maintainers/community can help.
3. Reference the message via its key rather than hardcoding literals.

### Commit & PR Guidelines

- Write clear commit messages that explain **why** more than **what**.
- Keep PRs small and focused; unrelated changes belong in separate PRs.
- Reference related issues (e.g. `Fixes #123`) in the PR description.
- Make sure `make vet test lint build` all pass before requesting review.
- New features should update the relevant docs (`README.md` and `README.en.md`)
  and, if applicable, `config.example.yaml`.

### Project Layout

See the "项目结构 / Project structure" section of the
[README](README.md#项目结构) for a detailed file-by-file map. In short:

- top-level `*.go` — CLI entry, config loading, signal handling.
- `internal/forward` — the pure-Go TCP/UDP forwarder.
- `internal/socat` — the system-`socat` fallback.
- `internal/i18n` — localization.

### License

By contributing, you agree that your contributions will be licensed under the
project's [Apache License 2.0](LICENSE).

---

## 贡献指南（中文）

### 贡献方式

- **报告缺陷** —— 提交一个 [issue](https://github.com/soulteary/portmap/issues)，
  附上清晰的描述和复现步骤。
- **提出建议** —— 提交 issue，说明使用场景与动机。
- **完善文档** —— 修正错别字、优化措辞或补充翻译。文档为中英双语
  （`README.md` / `README.en.md`），涉及内容改动时请尽量保持两份同步。
- **提交代码** —— 通过 Pull Request 修复缺陷或实现功能。

### 环境准备

- **Go 1.27+**（准确版本以 [`go.mod`](go.mod) 为准）。
- **make** —— 用于驱动常用的构建/测试/检查任务。
- **golangci-lint** `v2.13.2` —— 与 CI 使用的版本一致
  （见 [`.github/workflows/ci.yml`](.github/workflows/ci.yml)）。
- **socat**（可选）—— 仅在本地验证 `socat` 模式时需要。
- **goreleaser**（可选）—— 仅在本地试跑发布流程时需要。

### 快速开始

```bash
# 1. 在 GitHub 上 Fork，然后克隆你的 fork
git clone https://github.com/<your-username>/portmap.git
cd portmap

# 2. 添加 upstream 远端以便同步上游
git remote add upstream https://github.com/soulteary/portmap.git

# 3. 构建并运行测试
make build
make test
```

### 开发流程

1. 从 `main` 分支创建主题分支：

```bash
git switch -c fix/short-description
```

2. 进行改动，保持提交聚焦、自成一体。
3. 运行下方的本地检查（与 CI 保持一致）。
4. 推送分支并向 `main` 发起 Pull Request。

### 本地检查

在发起 PR 前请先运行以下命令，它们与 CI 的检查一致：

```bash
make vet    # go vet ./...
make test   # go test ./... -race -count=1
make lint   # golangci-lint run ./...
make build  # 注入版本信息编译
make security # govulncheck
```

CI 会在 Linux、macOS、Windows 上运行 `go vet` 与 `go test -race`，并单独运行
lint、模块整洁与漏洞检查；同时会交叉编译包括 `windows/arm64` 在内的全部发布目标。
请确保代码在各平台间保持可移植。

### 代码规范

- **格式化** —— 使用 `gofmt` / `goimports`，保持地道的 Go 风格。
- **静态检查** —— 代码需通过 `golangci-lint`（配置见
  [`.golangci.yml`](.golangci.yml)）。
- **测试** —— 任何行为变更都应新增或更新测试；新包应附带 `_test.go`。现有测试需在
  `-race` 下持续通过。
- **跨平台** —— 本项目同时面向类 Unix 系统与 Windows。平台相关代码使用构建标签与
  拆分文件实现（如 `signals_unix.go` / `signals_windows.go`、
  `reuseaddr_unix.go` / `reuseaddr_windows.go`）。请优先沿用该模式，而非运行时
  判断操作系统。
- **注释** —— 解释意图与取舍，而非复述显而易见的代码。

### 多语言（i18n）

面向用户的帮助文本、日志与错误消息在 `internal/i18n` 中做了本地化
（`en`/`zh`/`ja`/`ko`/`fr`/`de`）。当你新增或修改这类消息时：

1. 在 `internal/i18n/keys.go` 中新增消息 key 常量。
2. 在**每一个** `internal/i18n/messages_*.go` 文件中提供对应翻译。若某语言你无法
   翻译，可先用英文占位并在 PR 中说明，便于维护者/社区协助补全。
3. 通过 key 引用消息，而非硬编码字符串字面量。

### 提交与 PR 规范

- 编写清晰的提交信息，更多解释**为什么**而非**做了什么**。
- PR 尽量小而聚焦；不相关的改动请拆分到独立 PR。
- 在 PR 描述中关联相关 issue（如 `Fixes #123`）。
- 请求评审前，确保 `make vet test lint build` 全部通过。
- 新功能应同步更新相关文档（`README.md` 与 `README.en.md`），必要时更新
  `config.example.yaml`。

### 项目结构

详细的逐文件说明见 [README 的「项目结构」章节](README.md#项目结构)。简要来说：

- 顶层 `*.go` —— 命令行入口、配置加载、信号处理。
- `internal/forward` —— 纯 Go 的 TCP/UDP 转发器。
- `internal/socat` —— 调用系统 `socat` 的 fallback。
- `internal/i18n` —— 多语言支持。

### 许可证

提交贡献即表示你同意你的贡献将按照项目的
[Apache License 2.0](LICENSE) 授权。
