package radio

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

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/misc"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	pineconeConnections "github.com/matrix-org/pinecone/connections"
	pineconeMulticast "github.com/matrix-org/pinecone/multicast"
	pineconeRouter "github.com/matrix-org/pinecone/router"
	pineconeSessions "github.com/matrix-org/pinecone/sessions"
)

const PROTOCOL_NAME = "wraith-module-pinecomms"

type Radio struct {
	// The private key for this pinecone peer; effectively its "identity".
	PineconeIdentity ed25519.PrivateKey

	// A logger instance which is passed on to pinecone.
	Logger *log.Logger

	// The address to listen on for incoming pinecone connections. If this
	// is an empty string, the node does not listen for connections and
	// multicast is also disabled (so the node can only connect to peers
	// outbound and cannot receive peer connections).
	InboundAddr string

	// The address to listen on for inbound HTTP. This allows peers to connect
	// to this node over websockets and exposes a debugging endpoint if enabled
	// via `WebserverDebugPath`. Additional routes can be configured via
	// `WebserverHandlers`. The webserver is disabled if this option is an empty
	// string.
	WebserverAddr string

	// A path on the webserver to expose debugging information at. If this is an
	// empty string, the node does not expose debugging information. This setting
	// depends on the webserver being enabled.
	WebserverDebugPath string

	// Whether to advertise this peer on the local network via multicast. This allows
	// for peers to find each other locally but may require modifications to firewall
	// rules. This option is always disabled if `InboundAddr` is not set.
	UseMulticast bool

	// A list of pinecone nodes with known addresses which this node can connect to
	// for a more stable connection to the network.
	StaticPeers []string

	// Additional handlers added to the webserver. This option exists mainly for
	// efficiency, to allow nodes which also need to run a regular webserver to
	// use the one used by pinecone for websockets. This saves allocating another
	// port and other system resources.
	WebserverHandlers []WebserverHandler

	// INTERNAL

	// Once instances ensuring that each method is only executed once at a given time.
	startOnce   misc.CheckableOnce
	stopOnce    misc.CheckableOnce
	restartOnce misc.CheckableOnce

	// Internal communication channels.
	reqExit chan struct{}
	ackExit chan struct{}
}

type WebserverHandler struct {
	Path    string
	Handler http.Handler
}

// Start the pinecone radio.
func (pm *Radio) Start() *pineconeSessions.SessionProtocol {
	// Reset startOnce when this function exits.
	defer func() {
		pm.startOnce = misc.CheckableOnce{}
	}()

	var pSocket *pineconeSessions.SessionProtocol

	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Init some internal communication channels.
		pm.reqExit = make(chan struct{})
		pm.ackExit = make(chan struct{})

		// Keep track of any goroutines we start.
		var wg sync.WaitGroup

		// Create a context to kill any goroutines we start.
		ctx, ctxCancel := context.WithCancel(context.Background())

		//
		// Main pinecone stuff.
		//

		// Set up pinecone components.
		pRouter := pineconeRouter.NewRouter(pm.Logger, pm.PineconeIdentity)
		pQUIC := pineconeSessions.NewSessions(pm.Logger, pRouter, []string{PROTOCOL_NAME})
		pMulticast := pineconeMulticast.NewMulticast(pm.Logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		pSocket = pQUIC.Protocol(PROTOCOL_NAME)

		// Listen for inbound connections if a TCP listener was configured.
		if pm.InboundAddr != "" {
			wg.Add(1)

			go func(ctx context.Context, wg *sync.WaitGroup, pRouter *pineconeRouter.Router) {
				defer wg.Done()

				listenCfg := net.ListenConfig{}
				listener, err := listenCfg.Listen(ctx, "tcp", pm.InboundAddr)

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
		if pm.WebserverDebugPath != "" {
			httpRouter.HandleFunc(pm.WebserverDebugPath, pRouter.ManholeHandler)
		}

		// If additional handlers are configured for the webserver, add them.
		for _, handler := range pm.WebserverHandlers {
			httpRouter.PathPrefix(handler.Path).Handler(handler.Handler)
		}

		// Non-pinecone HTTP server.
		httpServer := http.Server{
			Addr:         pm.WebserverAddr,
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
		if pm.UseMulticast {
			wg.Add(1)
			pMulticast.Start()
		}

		// Connect to any static peers we were given.
		for _, peer := range pm.StaticPeers {
			pManager.AddPeer(peer)
		}

		go func() {
			// Wait until exit is requested.
			<-pm.reqExit

			// Kill all goroutines we spawned which have our context.
			ctxCancel()

			// Tear down non-pinecone HTTP server.
			httpShutdownTimeoutCtx, httpShutdownTimeoutCtxCancel := context.WithTimeout(context.Background(), time.Second*2)
			httpServer.Shutdown(httpShutdownTimeoutCtx)
			httpShutdownTimeoutCtxCancel()

			// Tear down pinecone components.
			if pm.UseMulticast {
				pMulticast.Stop()
				wg.Done()
			}
			pManager.RemovePeers()
			pQUIC.Close()
			//pRouter.Close()

			// Wait for all the goroutines we started to exit.
			wg.Wait()

			// Acknowledge the exit request.
			close(pm.ackExit)
		}()
	})

	return pSocket
}

// Stop the pinecone radio.
func (pm *Radio) Stop() {
	// Reset stopOnce when this function exits.
	defer func(pm *Radio) {
		pm.stopOnce = misc.CheckableOnce{}
	}(pm)

	// Only execute this once at a time.
	pm.stopOnce.Do(func() {
		// Only actually do anything if the radio is running, otherwise
		// we'll block forever because nothing would read from the channel.
		//
		// Theoretically the radio could exit after our check but before
		// we write to the channel which causes a race and results in a
		// deadlock. In practice this should be impossible as the radio
		// can only exit when this function is called and only one of this
		// function can run at a time. The guarantee could be made stronger
		// with locks but this isn't really worth the added complexity.
		if pm.IsRunning() {
			close(pm.reqExit)

			// Wait for the exit request to be acknowledged.
			<-pm.ackExit
		}
	})
}

// Restart the pinecone radio. Equivalent to calling Stop() and Start().
func (pm *Radio) Restart() {
	// Reset restartOnce when this function exits.
	defer func(pm *Radio) {
		pm.restartOnce = misc.CheckableOnce{}
	}(pm)

	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		pm.Stop()
		pm.Start()
	})
}

// Check whether the pinecone radio is currently running.
func (pm *Radio) IsRunning() bool {
	return pm.startOnce.Doing()
}

var initonce sync.Once

// Get an instance of the pinecone radio.
func GetInstance() *Radio {
	initonce.Do(func() {
		// Disable quic-go's debug message.
		os.Setenv("QUIC_GO_DISABLE_RECEIVE_BUFFER_WARNING", "true")
	})

	// Generate some default options.
	_, randomPineconeIdentity, randomPineconeIdentityErr := ed25519.GenerateKey(nil)
	if randomPineconeIdentityErr != nil {
		panic(fmt.Errorf("fatal error while generating pinecone identity for radio defaults: %e", randomPineconeIdentityErr))
	}

	return &Radio{
		PineconeIdentity:   randomPineconeIdentity,
		Logger:             log.New(io.Discard, "", 0),
		InboundAddr:        ":0",
		WebserverAddr:      ":0",
		WebserverDebugPath: "",
		UseMulticast:       false,
		StaticPeers:        []string{},
		WebserverHandlers:  []WebserverHandler{},
	}
}
