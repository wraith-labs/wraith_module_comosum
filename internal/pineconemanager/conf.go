package pineconemanager

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
