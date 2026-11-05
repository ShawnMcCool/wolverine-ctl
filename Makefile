PREFIX ?= /usr/local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test install install-udev uninstall clean

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o wolverine-ctl .

test:
	go vet ./...
	go test ./...

install: build
	install -Dm755 wolverine-ctl $(DESTDIR)$(PREFIX)/bin/wolverine-ctl

install-udev:
	install -Dm644 udev/70-wolverine-ctl.rules $(DESTDIR)/etc/udev/rules.d/70-wolverine-ctl.rules
	udevadm control --reload-rules
	udevadm trigger --subsystem-match=hidraw

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/wolverine-ctl $(DESTDIR)/etc/udev/rules.d/70-wolverine-ctl.rules

clean:
	rm -f wolverine-ctl
