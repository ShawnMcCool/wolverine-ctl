PREFIX ?= /usr/local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test dist install install-udev uninstall clean

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o wolverine-ctl .

test:
	go vet ./...
	go test ./...

# Static release binaries and checksums in dist/.
dist:
	rm -rf dist && mkdir -p dist
	for arch in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" \
			-o dist/wolverine-ctl-$(VERSION)-linux-$$arch . ; \
	done
	cd dist && sha256sum wolverine-ctl-* > SHA256SUMS

install: build
	install -Dm755 wolverine-ctl $(DESTDIR)$(PREFIX)/bin/wolverine-ctl

install-udev:
	install -Dm644 udev/70-wolverine-ctl.rules $(DESTDIR)/etc/udev/rules.d/70-wolverine-ctl.rules
	udevadm control --reload-rules
	udevadm trigger --subsystem-match=hidraw

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/wolverine-ctl $(DESTDIR)/etc/udev/rules.d/70-wolverine-ctl.rules

clean:
	rm -rf wolverine-ctl dist
