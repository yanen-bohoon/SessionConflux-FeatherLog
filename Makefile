## SessionConflux 统一构建
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
# LDFLAGS for unified binary
LDFLAGS := -s -w -X main.version=$(VERSION) -X github.com/wesm/agentsview/cmd/agentsview.version=$(VERSION) -X github.com/wesm/agentsview/cmd/agentsview.commit=$(COMMIT) -X github.com/wesm/agentsview/cmd/agentsview.buildDate=$(BUILD_DATE)
BIN := session-conflux
INSTALL_DIR := $(HOME)/SessionConflux-FeatherLog

.PHONY: build install clean release test desktop-macos-app

# 构建统一 binary (包含 Cloud Sync + Web UI)
build:
	CGO_ENABLED=1 go build -tags fts5 -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/session-conflux/

# 一键安装
install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/$(BIN)
	@echo "安装完成: $(INSTALL_DIR)/"
	@echo "  └── session-conflux"
	@echo ""
	@if ! echo "$$PATH" | grep -q "$(INSTALL_DIR)"; then \
		echo "提示: 将此目录添加到 PATH 以便全局使用:"; \
		echo "  export PATH=\"$(INSTALL_DIR):\$$PATH\""; \
	fi

test:
	go test ./...
	cd agentsview && go test -tags fts5 ./...

# 交叉编译 (仅 4 个常用平台)
release:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags fts5 -ldflags "$(LDFLAGS)" -o $(BIN)_darwin_arm64 ./cmd/session-conflux/
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags fts5 -ldflags "$(LDFLAGS)" -o $(BIN)_darwin_amd64 ./cmd/session-conflux/
	GOOS=linux  GOARCH=amd64 CGO_ENABLED=1 go build -tags fts5 -ldflags "$(LDFLAGS)" -o $(BIN)_linux_amd64  ./cmd/session-conflux/
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 go build -tags fts5 -ldflags "$(LDFLAGS)" -o $(BIN)_windows_amd64.exe ./cmd/session-conflux/

# 构建 macOS 桌面应用 (需 Rust + Node.js)
desktop-macos-app:
	cd agentsview/desktop/scripts && bash prepare-sidecar.sh
	cd agentsview/desktop && npx tauri build
	mkdir -p dist/desktop/macos
	cp -r agentsview/desktop/src-tauri/target/release/bundle/macos/SessionConflux.app dist/desktop/macos/
	@echo "App 输出: dist/desktop/macos/SessionConflux.app"

clean:
	rm -f $(BIN)
	rm -f $(BIN)_darwin_arm64 $(BIN)_darwin_amd64 $(BIN)_linux_amd64 $(BIN)_windows_amd64.exe
