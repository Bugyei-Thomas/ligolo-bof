module ligolo-ng-bof

go 1.24.0

toolchain go1.24.4

require (
	github.com/coder/websocket v1.8.14
	github.com/hashicorp/yamux v0.1.0
	github.com/nicocha30/ligolo-ng v0.0.0
	golang.org/x/net v0.49.0
)

require (
	github.com/go-ping/ping v1.1.0 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/shamaton/msgpack/v2 v2.2.3 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)

replace github.com/nicocha30/ligolo-ng => ../ligolo-ng
