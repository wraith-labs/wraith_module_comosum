package main

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
)

const (
	STATE_CLEANUP_INTERVAL     = 30 * time.Second
	STATE_CLIENT_EXPIRY_DELAY  = proto.HEARTBEAT_MARK_DEAD_DELAY
	STATE_REQUEST_EXPIRY_DELAY = 10 * time.Minute

	MAX_DATA_PAGE_SIZE = 200
)

type Config struct {
	// The address of the homeserver to connect to for C2.
	homeserver string

	// The username to authenticate to the HS with.
	username string

	// The password to authenticate to the HS with.
	password string

	// List of MXIDs with admin privileges over the C2.
	admins []string

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

	c.homeserver = os.Getenv("WMP_HOMESERVER")
	c.username = os.Getenv("WMP_USERNAME")
	c.password = os.Getenv("WMP_PASSWORD")
	c.admins = strings.Split(os.Getenv("WMP_ADMINS"), " ")
	c.pineconeId = os.Getenv("WMP_ID_PINECONE")
	c.logPinecone = logPinecone
	c.pineconeInboundTcpAddr = os.Getenv("WMP_INBOUND_TCP_PINECONE")
	c.pineconeInboundWebAddr = os.Getenv("WMP_INBOUND_WEB_PINECONE")
	c.pineconeDebugEndpoint = os.Getenv("WMP_DEBUG_ENDPOINT_PINECONE")
	c.pineconeUseMulticast = pineconeUseMulticast
	c.pineconeStaticPeers = os.Getenv("WMP_STATIC_PEERS_PINECONE")

	//
	// Validate config.
	//

	_, hsParseError := url.ParseRequestURI(c.homeserver)
	if hsParseError != nil {
		panic("could not parse homeserver url")
	}

	if c.username == "" || c.password == "" {
		panic("please provide homeserver username and password")
	}
}
