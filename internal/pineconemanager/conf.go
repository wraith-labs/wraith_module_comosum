package pineconemanager

import (
	"crypto/ed25519"
	"log"
	"net/http"
	"sync"
)

// This struct represents the state of the config at a given time. It should
// be treated as immutable.
type configSnapshot struct {
	// The private key for this pinecone peer; effectively its "identity".
	pineconeIdentity ed25519.PrivateKey

	// A logger instance which is passed on to pinecone.
	logger *log.Logger

	// The address to listen on for incoming pinecone connections. If this
	// is an empty string, the node does not listen for connections and
	// multicast is also disabled (so the node can only connect to peers
	// outbound and cannot receive peer connections).
	inboundAddr string

	// The address to listen on for inbound HTTP. This allows peers to connect
	// to this node over websockets and exposes a debugging endpoint if enabled
	// via `WebserverDebugPath`. Additional routes can be configured via
	// `WebserverHandlers`. The webserver is disabled if this option is an empty
	// string.
	webserverAddr string

	// A path on the webserver to expose debugging information at. If this is an
	// empty string, the node does not expose debugging information. This setting
	// depends on the webserver being enabled.
	webserverDebugPath string

	// Whether to advertise this peer on the local network via multicast. This allows
	// for peers to find each other locally but may require modifications to firewall
	// rules. This option is always disabled if `InboundAddr` is not set.
	useMulticast bool

	// A list of pinecone nodes with known addresses which this node can connect to
	// for a more stable connection to the network.
	staticPeers []string

	// Additional handlers added to the webserver. This option exists mainly for
	// efficiency, to allow nodes which also need to run a regular webserver to
	// use the one used by pinecone for websockets. This saves allocating another
	// port and other system resources.
	webserverHandlers map[string]http.Handler
}

// This struct represents the configuration for a pinecone manager. Values can be
// accessed and edited via their respective read and update methods.
type config struct {
	//
	// Internal data
	//

	lock sync.RWMutex

	//
	// Config options
	//

	configSnapshot
}

//
// Internal
//

// Lock the config for writing and return a function to unlock it.
func (c *config) autolock() func() {
	c.lock.Lock()
	return func() {
		c.lock.Unlock()
	}
}

// Lock the config for reading and return a function to unlock it.
func (c *config) autorlock() func() {
	c.lock.RLock()
	return func() {
		c.lock.RUnlock()
	}
}

//
// Misc
//

// This method returns a struct representing the current state of the config.
// It is unexported as it is only meant for use within the module.
func (c *config) snapshot() configSnapshot {
	defer c.autorlock()()

	return c.configSnapshot
}

//
// Setters
//

func (pm *manager) SetPineconeIdentity(u ed25519.PrivateKey) {
	defer pm.conf.autolock()()

	pm.conf.pineconeIdentity = u
}

func (pm *manager) SetLogger(u *log.Logger) {
	defer pm.conf.autolock()()

	pm.conf.logger = u
}

func (pm *manager) SetInboundAddr(u string) {
	defer pm.conf.autolock()()

	pm.conf.inboundAddr = u
}

func (pm *manager) SetWebserverAddr(u string) {
	defer pm.conf.autolock()()

	pm.conf.webserverAddr = u
}

func (pm *manager) SetWebserverDebugPath(u string) {
	defer pm.conf.autolock()()

	pm.conf.webserverDebugPath = u
}

func (pm *manager) SetUseMulticast(u bool) {
	defer pm.conf.autolock()()

	pm.conf.useMulticast = u
}

func (pm *manager) SetStaticPeers(u []string) {
	defer pm.conf.autolock()()

	pm.conf.staticPeers = u
}

func (pm *manager) SetWebserverHandlers(u map[string]http.Handler) {
	defer pm.conf.autolock()()

	pm.conf.webserverHandlers = u
}

//
// Getters
//

func (pm *manager) GetPineconeIdentity() ed25519.PrivateKey {
	defer pm.conf.autorlock()()

	return pm.conf.pineconeIdentity
}

func (pm *manager) GetLogger() *log.Logger {
	defer pm.conf.autorlock()()

	return pm.conf.logger
}

func (pm *manager) GetInboundAddr() string {
	defer pm.conf.autorlock()()

	return pm.conf.inboundAddr
}

func (pm *manager) GetWebserverAddr() string {
	defer pm.conf.autorlock()()

	return pm.conf.webserverAddr
}

func (pm *manager) GetWebserverDebugPath() string {
	defer pm.conf.autorlock()()

	return pm.conf.webserverDebugPath
}

func (pm *manager) GetUseMulticast() bool {
	defer pm.conf.autorlock()()

	return pm.conf.useMulticast
}

func (pm *manager) GetStaticPeers() []string {
	defer pm.conf.autorlock()()

	return pm.conf.staticPeers
}

func (pm *manager) GetWebserverHandlers() map[string]http.Handler {
	defer pm.conf.autorlock()()

	return pm.conf.webserverHandlers
}
