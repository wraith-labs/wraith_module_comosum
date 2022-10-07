package pmanager

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	txq     chan packet
	rxq     chan packet

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
		// Init some internal communication channels.
		managerInstance.reqExit = make(chan struct{})
		managerInstance.ackExit = make(chan struct{})

		// Grab a snapshot of the config (this ensures the
		// config is never in an inconsistent state when values
		// need to be read multiple times).
		c := pm.conf.snapshot()

		// Keep track of any goroutines we start.
		var wg sync.WaitGroup

		// Create a context to kill any goroutines we start.
		ctx, ctxCancel := context.WithCancel(context.Background())

		//
		// Main pinecone stuff.
		//

		// Set up pinecone components.
		pRouter := pineconeRouter.NewRouter(c.logger, c.pineconeIdentity)
		pQUIC := pineconeSessions.NewSessions(c.logger, pRouter, []string{"wraith"})
		pMulticast := pineconeMulticast.NewMulticast(c.logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		// Set up rx queue handling.
		pMux := mux.NewRouter().SkipClean(true).UseEncodedPath()
		pMux.PathPrefix(ROUTE_PREFIX).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the payload from request body.
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}

			// Try to JSON parse the payload.
			data := packetData{}
			err = json.Unmarshal(payload, &data)
			if err != nil {
				w.WriteHeader(400)
				return
			}

			// Fill out packet metadata.
			p := packet{
				Peer:   r.RemoteAddr,
				Method: r.Method,
				Route:  strings.TrimPrefix(r.URL.EscapedPath(), ROUTE_PREFIX),
				Data:   data,
			}

			// Respond so the requester doesn't have to wait for the queue to empty.
			w.WriteHeader(200)

			// Add to the queue.
			pm.rxq <- p
		})

		pHTTP := pQUIC.Protocol("wraith").HTTP()
		pHTTP.Mux().Handle("/", pMux)

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
			httpRouter.Handle(route, handler)
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
		}

		// Manage tx queue.
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()

			for {
				select {
				case p := <-pm.txq:

					// Serialize the payload from the queue element.
					payload, err := json.Marshal(p.Data)
					if err != nil {
						pm.conf.logger.Printf("failed to serialize tx queue element due to error: %e", err)
						continue
					}

					// Set up request to peer.
					req := http.Request{
						Method: p.Method,
						URL: &url.URL{
							Scheme: "http",
							Host:   p.Peer,
							Path:   ROUTE_PREFIX + p.Route,
						},
						Cancel: ctx.Done(),
						Body:   io.NopCloser(bytes.NewReader(payload)),
					}

					// Send request to peer.
					pHTTP.Client().Do(&req)

				case <-ctx.Done():
					// If the context is closed, exit.
					return
				}
			}
		}(ctx)

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

		// Acknowledge the exit request.
		close(pm.ackExit)
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
			close(pm.reqExit)

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
func (pm *manager) Send(ctx context.Context, p packet) error {
	select {
	case pm.txq <- p:
		return nil
	case <-pm.ackExit:
		return fmt.Errorf("manager exited while trying to send packet")
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while trying to send packet (%e)", ctx.Err())
	}
}

// Receive incoming packets. Blocks until either a packet is received or
// the provided context expires.
func (pm *manager) Recv(ctx context.Context) (packet, error) {
	select {
	case p := <-pm.rxq:
		return p, nil
	case <-pm.ackExit:
		return packet{}, fmt.Errorf("manager exited while trying to send packet")
	case <-ctx.Done():
		return packet{}, fmt.Errorf("context cancelled while trying to receive packet (%e)", ctx.Err())
	}
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
		managerInstance.txq = make(chan packet)
		managerInstance.rxq = make(chan packet)
	})

	return managerInstance
}
