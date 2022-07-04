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
	"sync"
	"time"

	"git.0x1a8510f2.space/wraith-labs/wraith-module-pinecomms/internal/misc"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	pineconeConnections "github.com/matrix-org/pinecone/connections"
	pineconeMulticast "github.com/matrix-org/pinecone/multicast"
	pineconeRouter "github.com/matrix-org/pinecone/router"
	pineconeSessions "github.com/matrix-org/pinecone/sessions"
	"github.com/sirupsen/logrus"
)

type pineconeManager struct {
	// Once instances ensuring that each method is only executed once at a given time.
	startOnce   misc.CheckableOnce
	stopOnce    misc.CheckableOnce
	restartOnce misc.CheckableOnce

	// Communication channels.
	reqExit chan struct{}
	ackExit chan struct{}

	// An array of config options for the manager and a lock to make it thread-safe.
	conf     [conf_option_count]any
	confLock sync.RWMutex
}

// Read a config option of the pinecone manager. This is thread-safe.
func (pm *pineconeManager) ConfGet(confId pineconeManagerConfOption) any {
	defer pm.confLock.RUnlock()
	pm.confLock.RLock()

	return pm.conf[confId]
}

// Set a config option of the pinecone manager. This is thead-safe. Note that the
// manager will need to be restarted if it's running for changes to take effect.
func (pm *pineconeManager) ConfSet(confId pineconeManagerConfOption, confVal any) error {
	defer pm.confLock.Unlock()
	pm.confLock.Lock()

	// Make sure we're not writing out-of-bounds (though this should never really
	// happen unless we did something wrong in this module specifically).
	if confId > conf_option_count-1 {
		return fmt.Errorf("config option %d does not exist", confId)
	}

	invalidTypeErr := fmt.Errorf("invalid type for config value %d", confId)

	// Validate config values before writing them.
	// TODO: Add extra validation where possible.
	switch confId {
	case CONF_PINECONE_IDENTITY:
		if _, ok := confVal.(ed25519.PrivateKey); !ok {
			return invalidTypeErr
		}
	case CONF_LOGGER:
		if _, ok := confVal.(*log.Logger); !ok {
			return invalidTypeErr
		}
	case CONF_INBOUND_ADDR:
		if _, ok := confVal.(*net.TCPAddr); !ok {
			return invalidTypeErr
		}
	case CONF_WEBSERVER_ADDR, CONF_WEBSERVER_DEBUG_PATH:
		if _, ok := confVal.(string); !ok {
			return invalidTypeErr
		}
	case CONF_USE_MULTICAST:
		if _, ok := confVal.(bool); !ok {
			return invalidTypeErr
		}
	case CONF_WRAPPED_PROTOS, CONF_STATIC_PEERS:
		if _, ok := confVal.([]string); !ok {
			return invalidTypeErr
		}
	case CONF_WEBSERVER_HANDLERS:
		if _, ok := confVal.(map[string]http.Handler); !ok {
			return invalidTypeErr
		}
	}

	// Update the config.
	pm.conf[confId] = confVal

	return nil
}

// Start the pinecone manager as configured. This blocks while the
// manager is running but can be started in a goroutine.
func (pm *pineconeManager) Start() {
	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Acknowledge the exit request that caused the manager to exit.
		// This MUST be the first defer as that means it gets executed last.
		defer func(pm *pineconeManager) {
			pm.ackExit <- struct{}{}
		}(pm)

		// Reset startOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.startOnce = misc.CheckableOnce{}
		}(pm)

		// Grab a snapshot of the config (this makes access to config
		// values easier and ensures the config is never in an inconsistent
		// state when values need to be read multiple times).
		privkey := pm.ConfGet(CONF_PINECONE_IDENTITY).(ed25519.PrivateKey)
		logger := pm.ConfGet(CONF_LOGGER).(*log.Logger)
		inboundAddr := pm.ConfGet(CONF_INBOUND_ADDR).(*net.TCPAddr)
		webserverAddr := pm.ConfGet(CONF_WEBSERVER_ADDR).(string)
		webserverDebugPath := pm.ConfGet(CONF_WEBSERVER_DEBUG_PATH).(string)
		useMulticast := pm.ConfGet(CONF_USE_MULTICAST).(bool)
		protos := pm.ConfGet(CONF_WRAPPED_PROTOS).([]string)
		staticPeers := pm.ConfGet(CONF_STATIC_PEERS).([]string)
		webserverHandlers := pm.ConfGet(CONF_WEBSERVER_HANDLERS).(map[string]http.Handler)

		// Keep track of any goroutines we start.
		var wg sync.WaitGroup

		// Create a context to kill any goroutines we start.
		ctx, ctxCancel := context.WithCancel(context.Background())

		//
		// Main pinecone stuff
		//

		// Set up pinecone components.
		pRouter := pineconeRouter.NewRouter(logger, privkey, false)
		pQUIC := pineconeSessions.NewSessions(logger, pRouter, protos)
		pMulticast := pineconeMulticast.NewMulticast(logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		if useMulticast {
			wg.Add(1)
			pMulticast.Start()
		}

		// Connect to any static peers we were given.
		for _, peer := range staticPeers {
			pManager.AddPeer(peer)
		}

		// Listen for inbound connections if a listener was configured.
		if inboundAddr != nil {
			wg.Add(1)
			go func(ctx context.Context, wg sync.WaitGroup) {
				listener, err := net.ListenTCP("tcp", inboundAddr)
				if err != nil {
					// TODO: Handle this?
					panic(fmt.Errorf("error while setting up inbound pinecone listener: %e", err))
				}

				for ctx.Err() == nil {
					// Don't block indefinitely on listener.Accept() so we can exit the
					// goroutine when the context in cancelled.
					listener.SetDeadline(time.Now().Add(time.Second))

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
				logrus.WithError(err).Error("Failed to upgrade WebSocket connection")
				return
			}
			conn := wrapWebSocketConn(c)
			if _, err = pRouter.Connect(
				conn,
				pineconeRouter.ConnectionZone("websocket"),
				pineconeRouter.ConnectionPeerType(pineconeRouter.PeerTypeRemote),
			); err != nil {
				logrus.WithError(err).Error("Failed to connect WebSocket peer to Pinecone switch")
			}
		})
		if webserverDebugPath != "" {
			httpRouter.HandleFunc(webserverDebugPath, pRouter.ManholeHandler)
		}

		pMux := mux.NewRouter().SkipClean(true).UseEncodedPath()
		pMux.PathPrefix("/ptest").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "HeloWorld Function") })

		pHTTP := pQUIC.Protocol("matrix").HTTP()
		pHTTP.Mux().Handle("/ptest", pMux)

		// Build both ends of a HTTP multiplex.
		httpServer := &http.Server{
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
			httpServer.Serve(pQUIC.Protocol("matrix"))
		}()
		go func() {
			httpBindAddr := fmt.Sprintf(":%d", webserverAddr)
			http.ListenAndServe(httpBindAddr, httpRouter)
		}()
		///////////////////////////////

		// Wait until exit is requested.
		for {
			select {
			case <-pm.reqExit:
				break
			}
		}

		// Kill all goroutines we spawned.
		ctxCancel()

		// Tear down pinecone.
		if useMulticast {
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
func (pm *pineconeManager) Stop() {
	// Only execute this once at a time.
	pm.stopOnce.Do(func() {
		// Reset stopOnce when this function exits.
		defer func(pm *pineconeManager) {
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
func (pm *pineconeManager) Restart() {
	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		// Reset restartOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.restartOnce = misc.CheckableOnce{}
		}(pm)

		pm.Stop()
		pm.Start()
	})
}

// Check whether the pinecone manager is currently running.
func (pm *pineconeManager) IsRunning() bool {
	return pm.startOnce.Doing()
}

var initonce sync.Once
var pineconeManagerInstance *pineconeManager = nil

// Get the instance of the pinecone manager. This instance is shared for
// the entire program and successive calls return the existing instance.
func GetInstance() *pineconeManager {
	// Create and initialise an instance of pineconeManager only once.
	initonce.Do(func() {
		pineconeManagerInstance = &pineconeManager{}

		// Generate some default options.

		_, randomPineconeIdentity, randomPineconeIdentityErr := ed25519.GenerateKey(nil)
		if randomPineconeIdentityErr != nil {
			panic(fmt.Errorf("fatal error while generating pinecone identity for pineconeManager defaults: %e", randomPineconeIdentityErr))
		}

		defaults := [conf_option_count]any{
			CONF_PINECONE_IDENTITY:    randomPineconeIdentity,
			CONF_LOGGER:               log.New(io.Discard, "", 0),
			CONF_INBOUND_ADDR:         ":0",
			CONF_WEBSERVER_ADDR:       ":0",
			CONF_WEBSERVER_DEBUG_PATH: "",
			CONF_USE_MULTICAST:        false,
			CONF_WRAPPED_PROTOS:       []string{},
			CONF_STATIC_PEERS:         []string{},
			CONF_WEBSERVER_HANDLERS:   map[string]http.Handler{},
		}

		// Set default config values to ensure that the config is never
		// in an unusable state and allow for sane options without setting
		// everything manually.
		pineconeManagerInstance.conf = defaults

		// Init communication channels.
		pineconeManagerInstance.reqExit = make(chan struct{})
		pineconeManagerInstance.ackExit = make(chan struct{})
	})

	return pineconeManagerInstance
}
