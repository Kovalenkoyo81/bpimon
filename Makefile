VERSION    ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%d)
LDFLAGS    := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)
BINARY     := bpimon
SERVER     ?=
GOOS       ?= linux
GOARCH     ?= arm64
GOARM      ?=

ifdef SERVER
  REMOTE := ssh $(SERVER)
else
  REMOTE :=
endif

.PHONY: install build deploy test vet logs follow status

# Install locally on this machine (run directly on the SBC)
install:
	@if [ "$$(id -u)" != "0" ]; then echo "Run as root: sudo -E make install"; exit 1; fi
	@if [ -z "$$BPIMON_TELEGRAM_TOKEN" ]; then echo "Error: BPIMON_TELEGRAM_TOKEN is not set"; exit 1; fi
	@if [ -z "$$BPIMON_TELEGRAM_CHATID" ]; then echo "Error: BPIMON_TELEGRAM_CHATID is not set"; exit 1; fi
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/bpimon/
	systemctl stop bpimon 2>/dev/null || true
	mkdir -p /etc/bpimon /var/lib/bpimon
	install -m 755 $(BINARY) /usr/local/bin/bpimon
	install -m 644 config.yaml /etc/bpimon/config.yaml
	install -m 644 deploy/bpimon.service /etc/systemd/system/bpimon.service
	@printf 'BPIMON_TELEGRAM_TOKEN=%s\nBPIMON_TELEGRAM_CHATID=%s\nBPIMON_TELEGRAM_ADMINS=%s\n' \
		"$$BPIMON_TELEGRAM_TOKEN" "$$BPIMON_TELEGRAM_CHATID" "$$BPIMON_TELEGRAM_ADMINS" \
		> /etc/bpimon/env && chmod 600 /etc/bpimon/env
	systemctl daemon-reload && systemctl enable bpimon && systemctl start bpimon
	@echo "bpimon $(VERSION) installed and started."

# Cross-compile binary (for deploy to a remote machine)
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(if $(GOARM),GOARM=$(GOARM)) go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/bpimon/

# Deploy to a remote machine (optional)
deploy: build
	@if [ -z "$(SERVER)" ]; then echo "Error: SERVER is not set"; exit 1; fi
	@if [ -z "$$BPIMON_TELEGRAM_TOKEN" ]; then echo "Error: BPIMON_TELEGRAM_TOKEN is not set"; exit 1; fi
	@if [ -z "$$BPIMON_TELEGRAM_CHATID" ]; then echo "Error: BPIMON_TELEGRAM_CHATID is not set"; exit 1; fi
	ssh $(SERVER) "systemctl stop bpimon 2>/dev/null; mkdir -p /etc/bpimon /var/lib/bpimon"
	scp $(BINARY) $(SERVER):/tmp/bpimon_new
	scp config.yaml $(SERVER):/etc/bpimon/config.yaml
	scp deploy/bpimon.service $(SERVER):/etc/systemd/system/bpimon.service
	@ssh $(SERVER) "printf 'BPIMON_TELEGRAM_TOKEN=%s\nBPIMON_TELEGRAM_CHATID=%s\nBPIMON_TELEGRAM_ADMINS=%s\n' \
		'$$BPIMON_TELEGRAM_TOKEN' '$$BPIMON_TELEGRAM_CHATID' '$$BPIMON_TELEGRAM_ADMINS' \
		> /etc/bpimon/env && chmod 600 /etc/bpimon/env"
	ssh $(SERVER) "mv /tmp/bpimon_new /usr/local/bin/bpimon && chmod +x /usr/local/bin/bpimon \
		&& systemctl daemon-reload && systemctl enable bpimon && systemctl start bpimon"

test:
	go test ./...

vet:
	go vet ./...

status:
	$(REMOTE) systemctl status bpimon --no-pager

logs:
	$(REMOTE) journalctl -u bpimon -n 50 --no-pager

follow:
	$(REMOTE) journalctl -u bpimon -f
