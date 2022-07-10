package pineconemanager

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"git.0x1a8510f2.space/wraith-labs/wraith-module-pinecomms/internal/misc"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	pineconeConnections "github.com/matrix-org/pinecone/connections"
	pineconeMulticast "github.com/matrix-org/pinecone/multicast"
	pineconeRouter "github.com/matrix-org/pinecone/router"
	pineconeSessions "github.com/matrix-org/pinecone/sessions"
)

type manager struct {
	// Once instances ensuring that each method is only executed once at a given time.
	startOnce   misc.CheckableOnce
	stopOnce    misc.CheckableOnce
	restartOnce misc.CheckableOnce

	// Communication channels.
	reqExit chan struct{}
	ackExit chan struct{}

	// A struct of config options for the manager with a lock to make it thread-safe.
	conf config
}

// Start the pinecone manager as configured. This blocks while the
// manager is running but can be started in a goroutine.
func (pm *manager) Start() {
	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Acknowledge the exit request that caused the manager to exit.
		// This MUST be the first defer as that means it gets executed last.
		defer func(pm *manager) {
			pm.ackExit <- struct{}{}
		}(pm)

		// Reset startOnce when this function exits.
		defer func(pm *manager) {
			pm.startOnce = misc.CheckableOnce{}
		}(pm)

		// Grab a snapshot of the config (this ensures the
		// config is never in an inconsistent state when values
		// need to be read multiple times).
		c := pm.conf.snapshot()

		// Keep track of any goroutines we start.
		var wg sync.WaitGroup

		// Create a context to kill any goroutines we start.
		ctx, ctxCancel := context.WithCancel(context.Background())

		//
		// Main pinecone stuff
		//

		// Set up pinecone components.
		pRouter := pineconeRouter.NewRouter(c.logger, c.pineconeIdentity, false)
		pQUIC := pineconeSessions.NewSessions(c.logger, pRouter, []string{"wraith"})
		pMulticast := pineconeMulticast.NewMulticast(c.logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		if c.useMulticast {
			wg.Add(1)
			pMulticast.Start()
		}

		// Connect to any static peers we were given.
		for _, peer := range c.staticPeers {
			pManager.AddPeer(peer)
		}

		// Listen for inbound connections if a listener was configured.
		if c.inboundAddr != "" {
			wg.Add(1)
			go func(ctx context.Context, wg sync.WaitGroup) {
				listenCfg := net.ListenConfig{}
				listener, err := listenCfg.Listen(ctx, "tcp", c.inboundAddr)

				if err != nil {
					// TODO: Handle this?
					panic(fmt.Errorf("error while setting up inbound pinecone listener: %e", err))
				}

				for ctx.Err() == nil {
					// Accept incoming connections. In case of error, drop connection but
					// otherwise ignore.
					conn, err := listener.Accept()
					if err != nil {
						continue
					}

					// Set up pinecone over the newly created connection.
					_, err = pRouter.Connect(
						conn,
						pineconeRouter.ConnectionPeerType(pineconeRouter.PeerTypeRemote),
					)

					// If pinecone setup failed, drop the connection.
					if err != nil {
						conn.Close()
						continue
					}
				}
				wg.Done()
			}(ctx, wg)
		}

		///////////////////////////////
		wsUpgrader := websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		}
		httpRouter := mux.NewRouter().SkipClean(true).UseEncodedPath()
		httpRouter.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			c, err := wsUpgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			conn := wrapWebSocketConn(c)
			if _, err = pRouter.Connect(
				conn,
				pineconeRouter.ConnectionZone("websocket"),
				pineconeRouter.ConnectionPeerType(pineconeRouter.PeerTypeRemote),
			); err != nil {
				// TODO: ?
			}
		})
		if c.webserverDebugPath != "" {
			httpRouter.HandleFunc(c.webserverDebugPath, pRouter.ManholeHandler)
		}

		pMux := mux.NewRouter().SkipClean(true).UseEncodedPath()
		pMux.PathPrefix("/ptest").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "HelloWorld Function") })

		pHTTP := pQUIC.Protocol("wraith").HTTP()
		pHTTP.Mux().Handle("/ptest", pMux)

		// Build both ends of a HTTP multiplex.
		httpServer := http.Server{
			Addr:         ":0",
			TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
			BaseContext: func(_ net.Listener) context.Context {
				return context.Background()
			},
			Handler: pMux,
		}

		go func() {
			httpServer.Serve(pQUIC.Protocol("wraith"))
		}()
		go func() {
			http.ListenAndServe(c.webserverAddr, httpRouter)
		}()
		///////////////////////////////

		// Wait until exit is requested.
	mainloop:
		for {
			select {
			case <-pm.reqExit:
				break mainloop
			}
		}

		// Kill all goroutines we spawned.
		ctxCancel()

		// Tear down pinecone.
		if c.useMulticast {
			pMulticast.Stop()
			wg.Done()
		}
		pManager.RemovePeers()
		pQUIC.Close()
		pRouter.Close()

		// Wait for all the goroutines we started to exit.
		wg.Wait()
	})
}

// Stop the pinecone manager.
func (pm *manager) Stop() {
	// Only execute this once at a time.
	pm.stopOnce.Do(func() {
		// Reset stopOnce when this function exits.
		defer func(pm *manager) {
			pm.stopOnce = misc.CheckableOnce{}
		}(pm)

		// Only actually do anything if the manager is running, otherwise
		// we'll block forever because nothing would read from the channel.
		//
		// Theoretically the manager could exit after our check but before
		// we write to the channel which causes a race and results in a
		// deadlock. In practice this should be impossible as the manager
		// can only exit when this function is called and only one of this
		// function can run at a time. The guarantee could be made stronger
		// with locks but this isn't really worth the added complexity.
		if pm.IsRunning() {
			pm.reqExit <- struct{}{}

			// Wait for the exit request to be acknowledged.
			<-pm.ackExit
		}
	})
}

// Restart the pinecone manager. Equivalent to calling Stop() and Start().
func (pm *manager) Restart() {
	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		// Reset restartOnce when this function exits.
		defer func(pm *manager) {
			pm.restartOnce = misc.CheckableOnce{}
		}(pm)

		pm.Stop()
		pm.Start()
	})
}

// Check whether the pinecone manager is currently running.
func (pm *manager) IsRunning() bool {
	return pm.startOnce.Doing()
}

var initonce sync.Once
var managerInstance *manager = nil

// Get the instance of the pinecone manager. This instance is shared for
// the entire program and successive calls return the existing instance.
func GetInstance() *manager {
	// Create and initialise an instance of manager only once.
	initonce.Do(func() {
		// Disable quic-go's debug message
		os.Setenv("QUIC_GO_DISABLE_RECEIVE_BUFFER_WARNING", "true")

		managerInstance = &manager{}

		// Generate some default options.

		_, randomPineconeIdentity, randomPineconeIdentityErr := ed25519.GenerateKey(nil)
		if randomPineconeIdentityErr != nil {
			panic(fmt.Errorf("fatal error while generating pinecone identity for manager defaults: %e", randomPineconeIdentityErr))
		}

		defaults := configSnapshot{
			pineconeIdentity:   randomPineconeIdentity,
			logger:             log.New(io.Discard, "", 0),
			inboundAddr:        ":0",
			webserverAddr:      ":0",
			webserverDebugPath: "",
			useMulticast:       false,
			staticPeers:        []string{},
			webserverHandlers:  map[string]http.Handler{},
		}

		// Set default config values to ensure that the config is never
		// in an unusable state and allow for sane options without setting
		// everything manually.
		managerInstance.conf.configSnapshot = defaults

		// Init communication channels.
		managerInstance.reqExit = make(chan struct{})
		managerInstance.ackExit = make(chan struct{})
	})

	return managerInstance
}
