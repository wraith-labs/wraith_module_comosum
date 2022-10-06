package pmanager

const (
	// The version of the wraith-module-pinecomms protocol supported by the current
	// version of the module. This is updated whenever a breaking change is made to
	// the protocol.
	//
	// https://web.archive.org/web/20220618041607/https://www.ssa.gov/oact/babynames/decades/century.html
	CURRENT_PROTO = "james"

	// The prefix for all Wraith PineComms HTTP routes.
	ROUTE_PREFIX = "wpc"
)

/*

The HTTP part of the protocol is extremely simple; only two routes are provided:

- PING: An echo request to check if the target is online. A 200 response indicates
	that everything is running as expected, any other status indicates that something
	is wrong, with optionally more details included in the response body as freeform data.

- SEND: An endpoint for sending signed JWT data.

*/

type route int

const (
	ROUTE_PING route = iota
	ROUTE_SEND
)

type packetData struct {
	// Wraith shm cells to read from.
	R []string

	// Wraith shm cells to write to and data to write.
	W map[string]any
}

type packet struct {
	// The pinecone peer this packet came from or is going to.
	Peer string

	// The HTTP method this packet was received or is to be sent with.
	Method string

	// The HTTP route this packet was received on or is being sent to.
	Route route

	// The data included with the packet encoded as pmanager-flavoured JWT.
	Data packetData
}
