LDFLAGS=-ldflags "-s -w"

all: linuxso_64

# ─────────────────────────  Linux targets  ─────────────────────────

linuxso_64:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build -buildmode=c-shared \
		-trimpath $(LDFLAGS) \
		-o ligolo.x64.so .
	@echo "[+] Built: ligolo.x64.so"

linuxso_32:
	CGO_ENABLED=1 GOOS=linux GOARCH=386 \
		go build -buildmode=c-shared \
		-trimpath $(LDFLAGS) \
		-o ligolo.x86.so .
	@echo "[+] Built: ligolo.x86.so"

# ─────────────────────────  Windows targets  ───────────────────────
# Requires cross-compilers: mingw-w64
#   sudo apt install gcc-mingw-w64-x86-64 gcc-mingw-w64-i686

windowsdll_64:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		CC=x86_64-w64-mingw32-gcc \
		go build -buildmode=c-shared \
		-trimpath $(LDFLAGS) \
		-o ligolo.x64.dll .
	@echo "[+] Built: ligolo.x64.dll"

windowsdll_32:
	CGO_ENABLED=1 GOOS=windows GOARCH=386 \
		CC=i686-w64-mingw32-gcc \
		go build -buildmode=c-shared \
		-trimpath $(LDFLAGS) \
		-o ligolo.x86.dll .
	@echo "[+] Built: ligolo.x86.dll"

# ─────────────────────────  Utilities  ─────────────────────────────

deps:
	go mod tidy
	@echo "[+] Dependencies resolved"

vet:
	CGO_ENABLED=1 go vet ./...
	@echo "[+] Vet passed"

clean:
	rm -f ligolo.x64.so ligolo.x86.so ligolo.x64.dll ligolo.x86.dll
	rm -f ligolo.x64.h ligolo.x86.h ligolo.x64.dll.h ligolo.x86.dll.h
	@echo "[+] Build artifacts removed"

symbols:
	@nm -D ligolo.x64.so 2>/dev/null | grep entrypoint || echo "Error: build linux target first"

.PHONY: all linuxso_64 linuxso_32 windowsdll_64 windowsdll_32 deps vet clean symbols
