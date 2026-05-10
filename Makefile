VERSION    ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%d)
LDFLAGS    := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)
BINARY     := bpimon
SERVER     ?= root@your-sbc-ip
GOOS       ?= linux
GOARCH     ?= arm
GOARM      ?= 7

.PHONY: build deploy test vet logs follow status

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/bpimon/

deploy: build
	@if [ -z "$$BPIMON_TELEGRAM_TOKEN" ]; then \
		echo "Error: BPIMON_TELEGRAM_TOKEN is not set"; exit 1; fi
	@if [ -z "$$BPIMON_TELEGRAM_CHATID" ]; then \
		echo "Error: BPIMON_TELEGRAM_CHATID is not set"; exit 1; fi
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

logs:
	ssh $(SERVER) "journalctl -u bpimon -n 50 --no-pager"

follow:
	ssh $(SERVER) "journalctl -u bpimon -f"

status:
	ssh $(SERVER) "systemctl status bpimon --no-pager"
