package proto

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
