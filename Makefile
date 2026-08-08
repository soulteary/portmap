BINARY  := portmap
PKG     := github.com/soulteary/portmap

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: all build test vet lint clean release snapshot

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./... -race -count=1

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINARY) dist/

# release 交叉编译多平台产物到 dist/。
# 注意：下面手写的平台矩阵需与 .goreleaser.yaml 中的 builds 保持同步。
# 正式发布以 GoReleaser 为准（见 make snapshot 与 .github/workflows/release.yml），
# 此目标仅用于本地快速交叉编译，不作为发布事实来源。
release:
	@mkdir -p dist
	@for target in \
		linux/amd64 linux/arm64 \
		darwin/amd64 darwin/arm64 \
		windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o dist/$(BINARY)-$$os-$$arch$$ext . ; \
	done

# snapshot 使用 GoReleaser 在本地试跑发布流程（不推送、不发布）。
snapshot:
	goreleaser release --snapshot --clean
