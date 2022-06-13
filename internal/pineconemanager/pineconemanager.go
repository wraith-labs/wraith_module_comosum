package pineconemanager

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

type pineconeManagerConfOption int

const (
	// The private key for this pinecone peer; effectively its "identity".
	CONF_PINECONE_IDENTITY pineconeManagerConfOption = iota

	// A logger instance which is passed on to pinecone.
	CONF_LOGGER

	// The address to listen on for incoming pinecone connections. If this
	// is an empty string, the node does not listen for connections and
	// multicast is also disabled (so the node can only connect to peers
	// outbound and cannot receive peer connections).
	CONF_INBOUND_ADDR

	// The address to listen on for inbound HTTP. This allows peers to connect
	// to this node over websockets and exposes a debugging endpoint if enabled
	// via `WebserverDebugPath`. Additional routes can be configured via
	// `WebserverHandlers`. The webserver is disabled if this option is an empty
	// string.
	CONF_WEBSERVER_ADDR

	// A path on the webserver to expose debugging information at. If this is an
	// empty string, the node does not expose debugging information. This setting
	// depends on the webserver being enabled.
	CONF_WEBSERVER_DEBUG_PATH

	// Whether to advertise this peer on the local network via multicast. This allows
	// for peers to find each other locally but may require modifications to firewall
	// rules. This option is always disabled if `InboundAddr` is not set.
	CONF_USE_MULTICAST

	// A list of protocols to advertise as supported by this node over pinecone.
	CONF_WRAPPED_PROTOS

	// A list of pinecone nodes with known addresses which this node can connect to
	// for a more stable connection to the network.
	CONF_STATIC_PEERS

	// Additional handlers added to the webserver. This option exists mainly for
	// efficiency, to allow nodes which also need to run a regular webserver to
	// use the one used by pinecone for websockets. This saves allocating another
	// port and other system resources.
	CONF_WEBSERVER_HANDLERS

	// Because this is at the bottom, it will automatically hold the value representing
	// the number of config options available. This is useful to create an array for
	// config options.
	conf_option_count
)

type pineconeManager struct {
	// Once instances ensuring that each method is only executed once at a given time.
	startOnce   sync.Once
	stopOnce    sync.Once
	restartOnce sync.Once

	// A context and related fields which control the lifetime of the pinecone manager.
	ctx       context.Context
	ctxCancel context.CancelFunc
	ctxLock   sync.RWMutex

	// An array of config options for the manager and a lock to make it thread-safe.
	conf     [conf_option_count]any
	confLock sync.RWMutex
}

// Read a config option of the pinecone manager. This is thread-safe.
func (pm *pineconeManager) ConfGet(confId pineconeManagerConfOption) (any, error) {
	defer pm.confLock.RUnlock()
	pm.confLock.RLock()

	// Make sure we're not writing out-of-bounds (though this should never really
	// happen unless we did something wrong in this module specifically).
	if confId > conf_option_count-1 {
		return nil, fmt.Errorf("config option %d does not exist", confId)
	}

	return pm.conf[confId], nil
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
	switch confId {
	case CONF_PINECONE_IDENTITY:
		if _, ok := confVal.(ed25519.PrivateKey); !ok {
			return invalidTypeErr
		}
	case CONF_LOGGER:
		if _, ok := confVal.(*log.Logger); !ok {
			return invalidTypeErr
		}
	case CONF_INBOUND_ADDR, CONF_WEBSERVER_ADDR, CONF_WEBSERVER_DEBUG_PATH:
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

	// Write the config
	pm.conf[confId] = confVal

	return nil
}

func (pm *pineconeManager) ConfSetDefaults() {
	_, randomPineconeIdentity, randomPineconeIdentityErr := ed25519.GenerateKey(nil)
	if randomPineconeIdentityErr != nil {
		panic(fmt.Errorf("fatal error while generating pinecone identity for pineconeManager defaults: %e", randomPineconeIdentityErr))
	}

	defaults := map[pineconeManagerConfOption]any{
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

	for key, value := range defaults {
		err := pm.ConfSet(key, value)
		if err != nil {
			panic(fmt.Errorf("fatal error while setting pineconeManager defaults: %e", err))
		}
	}
}

// Start the pinecone manager as configured. This blocks while the
// manager is running but can be started in a goroutine.
func (pm *pineconeManager) Start() {
	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Reset startOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.startOnce = sync.Once{}
		}(pm)

		// Set up the context to allow stopping the pinecone manager.
		pm.ctxLock.Lock()
		pm.ctx, pm.ctxCancel = context.WithCancel(context.Background())
		pm.ctxLock.Unlock()

		//
		// Main pinecone stuff
		//

		// Grab a snapshot of the config (this makes access to config
		// values easier and ensures the config is never in an inconsistent
		// state when values need to be read multiple times).
		privkey := misc.NoError(pm.ConfGet(CONF_PINECONE_IDENTITY)).(ed25519.PrivateKey)
		logger := misc.NoError(pm.ConfGet(CONF_LOGGER)).(*log.Logger)
		inboundAddr := misc.NoError(pm.ConfGet(CONF_INBOUND_ADDR)).(string)
		webserverAddr := misc.NoError(pm.ConfGet(CONF_WEBSERVER_ADDR)).(string)
		webserverDebugPath := misc.NoError(pm.ConfGet(CONF_WEBSERVER_DEBUG_PATH)).(string)
		useMulticast := misc.NoError(pm.ConfGet(CONF_USE_MULTICAST)).(bool)
		protos := misc.NoError(pm.ConfGet(CONF_WRAPPED_PROTOS)).([]string)
		staticPeers := misc.NoError(pm.ConfGet(CONF_STATIC_PEERS)).([]string)
		webserverHandlers := misc.NoError(pm.ConfGet(CONF_WEBSERVER_HANDLERS)).(map[string]http.Handler)

		pRouter := pineconeRouter.NewRouter(logger, privkey, false)
		pQUIC := pineconeSessions.NewSessions(logger, pRouter, protos)
		pMulticast := pineconeMulticast.NewMulticast(logger, pRouter)
		pManager := pineconeConnections.NewConnectionManager(pRouter, nil)

		if useMulticast {
			pMulticast.Start()
		}

		for _, peer := range staticPeers {
			pManager.AddPeer(peer)
		}

		if inboundAddr != "" {

		}

		///////////////////////////////
		go func() {
			listener, err := net.Listen("tcp", *instanceListen)
			if err != nil {
				panic(err)
			}

			fmt.Println("Listening on", listener.Addr())

			for {
				conn, err := listener.Accept()
				if err != nil {
					logrus.WithError(err).Error("listener.Accept failed")
					continue
				}

				port, err := pRouter.Connect(
					conn,
					pineconeRouter.ConnectionPeerType(pineconeRouter.PeerTypeRemote),
				)
				if err != nil {
					logrus.WithError(err).Error("pSwitch.Connect failed")
					continue
				}

				fmt.Println("Inbound connection", conn.RemoteAddr(), "is connected to port", port)
			}
		}()

		wsUpgrader := websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		}
		httpRouter := mux.NewRouter().SkipClean(true).UseEncodedPath()
		httpRouter.PathPrefix("/test").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "HeloWorld Function") })
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
		httpRouter.HandleFunc("/pinecone", pRouter.ManholeHandler)

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
			pubkey := pRouter.PublicKey()
			logrus.Info("Listening on ", hex.EncodeToString(pubkey[:]))
			logrus.Fatal(httpServer.Serve(pQUIC.Protocol("matrix")))
		}()
		go func() {
			httpBindAddr := fmt.Sprintf(":%d", *instancePort)
			logrus.Info("Listening on ", httpBindAddr)
			logrus.Fatal(http.ListenAndServe(httpBindAddr, httpRouter))
		}()
		///////////////////////////////

		for {
			select {
			case <-pm.ctx.Done():
				break
			}
		}

		if useMulticast {
			pMulticast.Stop()
		}
	})
}

// Stop the pinecone manager.
func (pm *pineconeManager) Stop() {
	// Only execute this once at a time.
	pm.stopOnce.Do(func() {
		// Reset stopOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.stopOnce = sync.Once{}
		}(pm)

		// Only actually do anything if the manager is running.
		// Note: This does not guarantee that the context is not cancelled
		// between the call to pm.IsRunning() and pm.ctxCancel(). A goroutine
		// could cancel the context after we check, which theoretically creates
		// a race condition. However, as a context CancelFunc is a no-op when
		// called multiple times, this is okay. The main reason for this check
		// is to prevent panics if the cancel func is nil which, it will be
		// before the manager's first run. As long as we know the manager
		// ran at some point (which this check guarantees), there won't be
		// issues.
		if pm.IsRunning() {
			pm.ctxCancel()
		}
	})
}

// Restart the pinecone manager. Equivalent to calling Stop() and Start().
func (pm *pineconeManager) Restart() {
	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		// Reset restartOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.restartOnce = sync.Once{}
		}(pm)

		pm.Stop()
		pm.Start()
	})
}

// Check whether the pinecone manager is currently running.
func (pm *pineconeManager) IsRunning() bool {
	// Make sure the context isn't modified while we're checking it.
	defer pm.ctxLock.RUnlock()
	pm.ctxLock.RLock()

	// If the context is nil, we're definitely not running.
	if pm.ctx == nil {
		return false
	}

	// If the context is not nil, we need to check if context.Err()
	// is nil to determine if the pm is running.
	if pm.ctx.Err() != nil {
		return false
	}

	return true
}

var initonce sync.Once
var pineconeManagerInstance *pineconeManager = nil

// Get the instance of the pinecone manager. This instance is shared for
// the entire program and successive calls return the existing instance.
func GetInstance() *pineconeManager {
	// Create and initialise an instance of pineconeManager only once.
	initonce.Do(func() {
		pineconeManagerInstance = &pineconeManager{}

		// Set default config values to ensure that the config is never
		// in an unusable state and allow for sane options without setting
		// everything manually.
		pineconeManagerInstance.ConfSetDefaults()
	})

	return pineconeManagerInstance
}
