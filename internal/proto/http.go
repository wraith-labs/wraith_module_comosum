package proto

/*

The HTTP part of the protocol is extremely simple; only three routes are provided:

- HEARTBEAT: Wraiths hit this endpoint on the c2 to report their status.
- REQUEST: The c2 hits this endpoint to send data to a Wraith.
- RESPONSE: The Wraith hits this endpoint to respond to c2's request.

*/

const (
	ROUTE_HEARTBEAT = "hb"
	ROUTE_REQUEST   = "rq"
	ROUTE_RESPONSE  = "rs"
)
