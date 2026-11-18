# traversal — Wails GUI. Needs CGO + WebKitGTK 4.1 (Arch: webkit2gtk-4.1).
# Tag is `webkit2_41` (Wails v2 build tag for libwebkit2gtk-4.1).

TAGS    := webkit2_41
WAILS   ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
BIN     := $(CURDIR)/build/bin/traversal
DESKTOP := $(HOME)/.local/share/applications/traversal.desktop

.PHONY: build dev check install uninstall clean

build:
	$(WAILS) build -tags "$(TAGS)" -clean

dev:
	$(WAILS) dev -tags "$(TAGS)"

check:
	go build ./...

# Build, then register a desktop entry (and a ~/.local/bin symlink) pointing
# at build/bin/traversal. Path= keeps the repo's .env loadable (godotenv reads CWD).
install: build
	mkdir -p $(HOME)/.local/bin $(dir $(DESKTOP))
	ln -sf $(BIN) $(HOME)/.local/bin/traversal
	printf '%s\n' \
		'[Desktop Entry]' \
		'Name=Traversal' \
		'Comment=EVE Online Navigation and Scanning Tool' \
		'Exec=$(BIN)' \
		'Path=$(CURDIR)' \
		'Icon=$(CURDIR)/build/appicon.png' \
		'Type=Application' \
		'Categories=Game;Utility;' \
		'StartupWMClass=traversal' > $(DESKTOP)
	-update-desktop-database $(dir $(DESKTOP)) 2>/dev/null

uninstall:
	rm -f $(HOME)/.local/bin/traversal $(DESKTOP)

clean:
	rm -rf build/bin frontend/dist/* frontend/wailsjs
