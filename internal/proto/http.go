package proto

/*

The HTTP part of the protocol is extremely simple; only two routes are provided:

- HEARTBEAT: Wraiths hit this endpoint on the c2 to report their status.
- REQUEST: The c2 hits this endpoint to send data to a Wraith.

*/

const (
	ROUTE_HEARTBEAT = "hb"
	ROUTE_REQUEST   = "rq"
)
