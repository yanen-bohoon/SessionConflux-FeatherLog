## Release cross-compilation
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN := session-conflux

.PHONY: build release clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/session-conflux/

release:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_darwin_arm64 ./cmd/session-conflux/
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_darwin_amd64 ./cmd/session-conflux/
	GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_linux_amd64  ./cmd/session-conflux/
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)_windows_amd64.exe ./cmd/session-conflux/

clean:
	rm -f $(BIN) $(BIN)_darwin_arm64 $(BIN)_darwin_amd64 $(BIN)_linux_amd64 $(BIN)_windows_amd64.exe
