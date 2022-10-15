package main

import (
	"os"
	"strconv"
	"sync/atomic"
)

const (
	DEFAULT_PANEL_LISTEN_ADDR = "127.0.0.1:48080"
	DEFAULT_PANEL_ADMIN_TOKEN = "wraith!"

	STARTING_ATTEMPTS_UNTIL_LOCKOUT = 5
)

type Config struct {
	// The address on which the control panel should listen for connections.
	panelListenAddr string

	// A string which allows administrative access to the panel.
	panelAdminToken string

	// A string which allows view-only access to the panel.
	panelViewToken string

	// Private key to use as identity on the pinecone network.
	pineconeId string

	// Whether to print pinecone's internal logs to stdout.
	logPinecone bool

	// The address to listen for inbound pinecone connections on.
	pineconeInboundTcpAddr string

	// The address to listen for inbound HTTP connections on (allows pinecone over websocket and pinecone debug endpoint if enabled).
	pineconeInboundWebAddr string

	// The HTTP path of the pinecone debug endpoint on the pinecone webserver (omit to disable).
	pineconeDebugEndpoint string

	// Comma-delimeted list of static peers to connect to.
	pineconeUseMulticast bool

	// Whether to use multicast to discover pinecone peers on the local network.
	pineconeStaticPeers string

	// Not configurable: the amount of failed login attempts until the panel is locked out.
	attemptsUntilLockout atomic.Int64
}

func (c *Config) Setup() {
	//
	// Pre-process env vars which need it.
	//

	logPinecone, _ := strconv.ParseBool(os.Getenv("WMP_LOG_PINECONE"))
	pineconeUseMulticast, _ := strconv.ParseBool(os.Getenv("WMP_USE_MULTICAST_PINECONE"))

	//
	// Save relevant env to config.
	//

	c.panelListenAddr = os.Getenv("WMP_PANEL_ADDR")
	c.panelAdminToken = os.Getenv("WMP_ADMIN_TOKEN")
	c.panelViewToken = os.Getenv("WMP_VIEW_TOKEN")
	c.pineconeId = os.Getenv("WMP_ID_PINECONE")
	c.logPinecone = logPinecone
	c.pineconeInboundTcpAddr = os.Getenv("WMP_INBOUND_TCP_PINECONE")
	c.pineconeInboundWebAddr = os.Getenv("WMP_INBOUND_WEB_PINECONE")
	c.pineconeDebugEndpoint = os.Getenv("WMP_DEBUG_ENDPOINT_PINECONE")
	c.pineconeUseMulticast = pineconeUseMulticast
	c.pineconeStaticPeers = os.Getenv("WMP_STATIC_PEERS_PINECONE")

	//
	// Set non-env config.
	//

	c.attemptsUntilLockout.Store(STARTING_ATTEMPTS_UNTIL_LOCKOUT)

	//
	// Validate config.
	//

	if c.panelListenAddr == "" {
		c.panelListenAddr = DEFAULT_PANEL_LISTEN_ADDR
	}

	if c.panelAdminToken == "" {
		c.panelAdminToken = DEFAULT_PANEL_ADMIN_TOKEN
	}
}
