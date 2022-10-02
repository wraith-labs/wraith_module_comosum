package pmanager

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

	// Internal communication channels.
	reqExit chan struct{}
	ackExit chan struct{}
	tx      chan struct{}
	rx      chan struct{}

	// A struct of config options for the manager with a lock to make it thread-safe.
	conf config
}

// Start the pinecone manager as configured. This blocks while the
// manager is running but can be started in a goroutine.
func (pm *manager) Start() {
	// Reset startOnce when this function exits.
	defer func() {
		pm.startOnce = misc.CheckableOnce{}
	}()

	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Acknowledge the exit request that caused the manager to exit.
		// This MUST be the first defer as that means it gets executed last.
		defer func() {
			// Catch and re-raise panics as otherwise they could possibly
			// block on the channel send below.
			if err := recover(); err != nil {
				panic(err)
			}

			pm.ackExit <- struct{}{}
		}()

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
		pRouter := pineconeRouter.NewRouter(c.logger, c.pineconeIdentity)
		pQUIC := pineconeSessions.NewSessions(c.logger, pRouter, []string{"wraith"})
		pMulticast := pineconeMulticast.NewMulticast(c.logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		// Set up pinecone HTTP paths.
		pMux := mux.NewRouter().SkipClean(true).UseEncodedPath()
		pMux.PathPrefix("/ptest").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			println("HELLO")
			w.WriteHeader(200)
		})

		pHTTP := pQUIC.Protocol("wraith").HTTP()
		pHTTP.Mux().Handle("/ptest", pMux)

		// Pinecone HTTP server.
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

			pineconeHttpServer.Serve(pQUIC.Protocol("wraith"))
		}()

		// Listen for inbound connections if a TCP listener was configured.
		if c.inboundAddr != "" {
			wg.Add(1)

			go func(ctx context.Context, wg *sync.WaitGroup, pRouter *pineconeRouter.Router) {
				defer wg.Done()

				listenCfg := net.ListenConfig{}
				listener, err := listenCfg.Listen(ctx, "tcp", c.inboundAddr)

				if err != nil {
					// TODO: Handle this?
					panic(fmt.Errorf("error while setting up inbound pinecone listener: %e", err))
				}

				for ctx.Err() == nil {
					// Make sure the below accept call does not block past context cancellation.
					go func() {
						<-ctx.Done()
						// TODO: Do we want to handle this error?
						listener.Close()
					}()

					// Accept incoming connections. In case of error, drop connection.
					conn, err := listener.Accept()
					if err != nil {
						if conn != nil {
							_ = conn.Close()
						}

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
			}(ctx, &wg, pRouter)
		}

		// Set up non-pinecone HTTP server.
		httpRouter := mux.NewRouter().SkipClean(true).UseEncodedPath()

		// Disable CORS.
		wsUpgrader := websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		}

		// Set up WebSocket peering route.
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
				return
			}
		})

		// If a webserver debug path is specified, set up pinecone manhole at that path.
		if c.webserverDebugPath != "" {
			httpRouter.HandleFunc(c.webserverDebugPath, pRouter.ManholeHandler)
		}

		// If additional handlers are configured for the webserver, add them.
		for route, handler := range c.webserverHandlers {
			httpRouter.HandleFunc(route, handler)
		}

		// Non-pinecone HTTP server.
		httpServer := http.Server{
			Addr:         c.webserverAddr,
			TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
			BaseContext: func(_ net.Listener) context.Context {
				return ctx
			},
			Handler: httpRouter,
		}

		// Start non-pinecone HTTP server in goroutine.
		wg.Add(1)
		go func() {
			defer wg.Done()

			httpServer.ListenAndServe()
		}()

		// Set up multicast discovery if enabled.
		if c.useMulticast {
			wg.Add(1)
			pMulticast.Start()
		}

		// Connect to any static peers we were given.
		for _, peer := range c.staticPeers {
			pManager.AddPeer(peer)
			/*resp, err := pHTTP.Client().Get("http://3d5054f46a6cbb6526a9e892151714e22ade8ff9e3a60fe534d991428936bbdf/ptest")
			if err != nil {
				fmt.Print(err)
			} else {
				var respbytes []byte
				resp.Body.Read(respbytes)
				fmt.Printf("%v\n%v\n", resp.StatusCode, respbytes)
			}*/
		}

		// Wait until exit is requested.
		<-pm.reqExit

		// Kill all goroutines we spawned which have our context.
		ctxCancel()

		// Tear down non-pinecone HTTP server.
		httpShutdownTimeoutCtx, httpShutdownTimeoutCtxCancel := context.WithTimeout(context.Background(), time.Second*2)
		httpServer.Shutdown(httpShutdownTimeoutCtx)
		httpShutdownTimeoutCtxCancel()

		// Tear down pinecone HTTP server.
		phttpShutdownTimeoutCtx, phttpShutdownTimeoutCtxCancel := context.WithTimeout(context.Background(), time.Second*2)
		pineconeHttpServer.Shutdown(phttpShutdownTimeoutCtx)
		phttpShutdownTimeoutCtxCancel()

		// Tear down pinecone components.
		if c.useMulticast {
			pMulticast.Stop()
			wg.Done()
		}
		pManager.RemovePeers()
		pQUIC.Close()
		//pRouter.Close()

		// Wait for all the goroutines we started to exit.
		wg.Wait()
	})
}

// Stop the pinecone manager.
func (pm *manager) Stop() {
	// Reset stopOnce when this function exits.
	defer func(pm *manager) {
		pm.stopOnce = misc.CheckableOnce{}
	}(pm)

	// Only execute this once at a time.
	pm.stopOnce.Do(func() {
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
	// Reset restartOnce when this function exits.
	defer func(pm *manager) {
		pm.restartOnce = misc.CheckableOnce{}
	}(pm)

	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		pm.Stop()
		pm.Start()
	})
}

// Send a given packet to a specific peer.
func (pm *manager) Send(peer string, packet packetOuter) error {
	return nil
}

// Ping a specific peer.
func (pm *manager) Ping(peer string) error {
	return nil
}

// Poll for incoming packets. Blocks until either a packet is received or
// the provided context expires.
func (pm *manager) Poll(ctx context.Context) (packetInner, error) {
	return packetInner{}, nil
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
			webserverHandlers:  map[string]http.HandlerFunc{},
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
