package radio

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
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

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/misc"
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	pineconeConnections "github.com/matrix-org/pinecone/connections"
	pineconeMulticast "github.com/matrix-org/pinecone/multicast"
	pineconeRouter "github.com/matrix-org/pinecone/router"
	pineconeSessions "github.com/matrix-org/pinecone/sessions"
)

type Radio interface {
	GetInboundAddr() string
	GetLogger() *log.Logger
	GetPineconeIdentity() ed25519.PrivateKey
	GetStaticPeers() []string
	GetUseMulticast() bool
	GetWebserverAddr() string
	GetWebserverDebugPath() string
	GetWebserverHandlers() []WebserverHandler
	IsRunning() bool
	Recv(ctx context.Context) (proto.Packet, error)
	RecvChan(ctx context.Context) chan proto.Packet
	Restart()
	Send(ctx context.Context, p proto.Packet) error
	SetInboundAddr(u string)
	SetLogger(u *log.Logger)
	SetPineconeIdentity(u ed25519.PrivateKey)
	SetStaticPeers(u []string)
	SetUseMulticast(u bool)
	SetWebserverAddr(u string)
	SetWebserverDebugPath(u string)
	SetWebserverHandlers(u []WebserverHandler)
	Start()
	Stop()
}

const PROTOCOL_NAME = "wraith-module-pinecomms"

type radio struct {
	// Once instances ensuring that each method is only executed once at a given time.
	startOnce   misc.CheckableOnce
	stopOnce    misc.CheckableOnce
	restartOnce misc.CheckableOnce

	// Internal communication channels.
	reqExit chan struct{}
	ackExit chan struct{}
	txq     chan proto.Packet
	rxq     chan proto.Packet

	// A struct of config options for the radio with a lock to make it thread-safe.
	conf config
}

// Start the pinecone radio as configured. This blocks while the
// radio is running but can be started in a goroutine.
func (pm *radio) Start() {
	// Reset startOnce when this function exits.
	defer func() {
		pm.startOnce = misc.CheckableOnce{}
	}()

	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Init some internal communication channels.
		radioInstance.reqExit = make(chan struct{})
		radioInstance.ackExit = make(chan struct{})

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
		pQUIC := pineconeSessions.NewSessions(c.logger, pRouter, []string{PROTOCOL_NAME})
		pMulticast := pineconeMulticast.NewMulticast(c.logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		// Set up rx queue handling.
		pMux := mux.NewRouter().SkipClean(true).UseEncodedPath()
		pMux.PathPrefix(proto.ROUTE_PREFIX).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read the payload from request body.
			data, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Fill out packet metadata.
			p := proto.Packet{
				Peer:   r.RemoteAddr,
				Method: r.Method,
				Route:  strings.TrimPrefix(r.URL.EscapedPath(), proto.ROUTE_PREFIX),
				Data:   data,
			}

			// Respond so the requester doesn't have to wait for the queue to empty.
			w.WriteHeader(http.StatusNoContent)

			// Add to the queue.
			pm.rxq <- p
		})

		pHTTP := pQUIC.Protocol(PROTOCOL_NAME).HTTP()
		pHTTP.Mux().Handle("/", pMux)

		app := fiber.New(fiber.Config{
			DisableStartupMessage:        true,
			DisablePreParseMultipartForm: true,
			ReadTimeout:                  10 * time.Second,
			WriteTimeout:                 10 * time.Second,
			IdleTimeout:                  10 * time.Second,
		})

		// Pinecone HTTP server.
		pineconeHttpServer := app.Server()

		// Start pinecone HTTP server in goroutine.
		wg.Add(1)
		go func() {
			defer wg.Done()

			pineconeHttpServer.Serve(pQUIC.Protocol(PROTOCOL_NAME))
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
		for _, handler := range c.webserverHandlers {
			httpRouter.PathPrefix(handler.Path).Handler(handler.Handler)
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

					// Set up request to peer.
					req := http.Request{
						Method: p.Method,
						URL: &url.URL{
							Scheme: "http",
							Host:   p.Peer,
							Path:   proto.ROUTE_PREFIX + p.Route,
						},
						Cancel: ctx.Done(),
						Body:   io.NopCloser(bytes.NewReader(p.Data)),
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
		pineconeHttpServer.ShutdownWithContext(phttpShutdownTimeoutCtx)
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

// Stop the pinecone radio.
func (pm *radio) Stop() {
	// Reset stopOnce when this function exits.
	defer func(pm *radio) {
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
func (pm *radio) Restart() {
	// Reset restartOnce when this function exits.
	defer func(pm *radio) {
		pm.restartOnce = misc.CheckableOnce{}
	}(pm)

	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		pm.Stop()
		pm.Start()
	})
}

// Send a given packet to a specific peer.
func (pm *radio) Send(ctx context.Context, p proto.Packet) error {
	select {
	case pm.txq <- p:
		return nil
	case <-pm.ackExit:
		return fmt.Errorf("radio exited while trying to send packet")
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while trying to send packet (%e)", ctx.Err())
	}
}

// Receive incoming packets. Blocks until either a packet is received or
// the provided context expires.
func (pm *radio) Recv(ctx context.Context) (proto.Packet, error) {
	select {
	case p := <-pm.rxq:
		return p, nil
	case <-pm.ackExit:
		return proto.Packet{}, fmt.Errorf("radio exited while trying to receive packet")
	case <-ctx.Done():
		return proto.Packet{}, fmt.Errorf("context cancelled while trying to receive packet (%e)", ctx.Err())
	}
}

// Receive incoming packets from a channel.
func (pm *radio) RecvChan(ctx context.Context) chan proto.Packet {
	c := make(chan proto.Packet)
	go func() {
		defer func() {
			close(c)
		}()
		for {
			packet, err := pm.Recv(ctx)
			if err != nil {
				return
			}
			c <- packet
		}
	}()
	return c
}

// Check whether the pinecone radio is currently running.
func (pm *radio) IsRunning() bool {
	return pm.startOnce.Doing()
}

var initonce sync.Once
var radioInstance *radio = nil

// Get the instance of the pinecone radio. This instance is shared for
// the entire program and successive calls return the existing instance.
func GetInstance() Radio {
	// Create and initialise an instance of radio only once.
	initonce.Do(func() {
		// Disable quic-go's debug message
		os.Setenv("QUIC_GO_DISABLE_RECEIVE_BUFFER_WARNING", "true")

		radioInstance = &radio{}

		// Generate some default options.

		_, randomPineconeIdentity, randomPineconeIdentityErr := ed25519.GenerateKey(nil)
		if randomPineconeIdentityErr != nil {
			panic(fmt.Errorf("fatal error while generating pinecone identity for radio defaults: %e", randomPineconeIdentityErr))
		}

		defaults := configSnapshot{
			pineconeIdentity:   randomPineconeIdentity,
			logger:             log.New(io.Discard, "", 0),
			inboundAddr:        ":0",
			webserverAddr:      ":0",
			webserverDebugPath: "",
			useMulticast:       false,
			staticPeers:        []string{},
			webserverHandlers:  []WebserverHandler{},
		}

		// Set default config values to ensure that the config is never
		// in an unusable state and allow for sane options without setting
		// everything manually.
		radioInstance.conf.configSnapshot = defaults

		// Init communication channels.
		radioInstance.txq = make(chan proto.Packet)
		radioInstance.rxq = make(chan proto.Packet)
	})

	return radioInstance
}
