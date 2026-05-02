## SessionConflux + AgentsView 统一构建
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN := session-conflux
AV_BIN := agentsview
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build build-av build-all install clean release

# 仅构建 session-conflux
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/session-conflux/

# 构建 AgentsView（含前端，从 vendored 源码）
build-av:
	$(MAKE) -C agentsview build-release
	cp agentsview/agentsview $(AV_BIN)

# 构建全部
build-all: build build-av

# 一键安装到 ~/.local/bin
install: build-all
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/$(BIN)
	cp $(AV_BIN) $(INSTALL_DIR)/$(AV_BIN)
	@echo "安装完成:"
	@echo "  session-conflux: $(INSTALL_DIR)/$(BIN)"
	@echo "  agentsview:      $(INSTALL_DIR)/$(AV_BIN)"
	@echo ""
	@if ! echo "$$PATH" | grep -q "$(INSTALL_DIR)"; then \
		echo "注意: $(INSTALL_DIR) 不在 PATH 中，请添加到 shell 配置:"; \
		echo "  export PATH=\"$(INSTALL_DIR):\$$PATH\""; \
	fi

# 交叉编译 session-conflux（4个平台）
release:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_darwin_arm64 ./cmd/session-conflux/
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_darwin_amd64 ./cmd/session-conflux/
	GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_linux_amd64  ./cmd/session-conflux/
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_windows_amd64.exe ./cmd/session-conflux/

clean:
	rm -f $(BIN) $(AV_BIN)
	rm -f $(BIN)_darwin_arm64 $(BIN)_darwin_amd64 $(BIN)_linux_amd64 $(BIN)_windows_amd64.exe
