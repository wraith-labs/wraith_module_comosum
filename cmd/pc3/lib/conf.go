package lib

import (
	"net/url"
	"os"
	"strconv"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
)

const (
	STATE_CLEANUP_INTERVAL     = 30 * time.Second
	STATE_CLIENT_EXPIRY_DELAY  = proto.HEARTBEAT_MARK_DEAD_DELAY
	STATE_REQUEST_EXPIRY_DELAY = 10 * time.Minute

	DATA_PAGE_SIZE = 10
)

type Config struct {
	// The address of the homeserver to connect to for C2.
	Homeserver string

	// The username to authenticate to the HS with.
	Username string

	// The password to authenticate to the HS with.
	Password string

	// The Matrix room where administration occurs.
	AdminRoom string

	// Private key to use as identity on the pinecone network.
	PineconeId string

	// Whether to print pinecone's internal logs to stdout.
	LogPinecone bool

	// The address to listen for inbound pinecone connections on.
	PineconeInboundTcpAddr string

	// The address to listen for inbound HTTP connections on (allows pinecone over websocket and pinecone debug endpoint if enabled).
	PineconeInboundWebAddr string

	// The HTTP path of the pinecone debug endpoint on the pinecone webserver (omit to disable).
	PineconeDebugEndpoint string

	// Comma-delimeted list of static peers to connect to.
	PineconeUseMulticast bool

	// Whether to use multicast to discover pinecone peers on the local network.
	PineconeStaticPeers string
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

	c.Homeserver = os.Getenv("WMP_HOMESERVER")
	c.Username = os.Getenv("WMP_USERNAME")
	c.Password = os.Getenv("WMP_PASSWORD")
	c.AdminRoom = os.Getenv("WMP_ADMIN_ROOM")
	c.PineconeId = os.Getenv("WMP_ID_PINECONE")
	c.LogPinecone = logPinecone
	c.PineconeInboundTcpAddr = os.Getenv("WMP_INBOUND_TCP_PINECONE")
	c.PineconeInboundWebAddr = os.Getenv("WMP_INBOUND_WEB_PINECONE")
	c.PineconeDebugEndpoint = os.Getenv("WMP_DEBUG_ENDPOINT_PINECONE")
	c.PineconeUseMulticast = pineconeUseMulticast
	c.PineconeStaticPeers = os.Getenv("WMP_STATIC_PEERS_PINECONE")

	//
	// Validate config.
	//

	_, hsParseError := url.ParseRequestURI(c.Homeserver)
	if hsParseError != nil {
		panic("could not parse homeserver url")
	}

	if c.Username == "" || c.Password == "" {
		panic("please provide homeserver credentials")
	}

	if c.AdminRoom == "" {
		panic("please provide an admin room id")
	}
}
