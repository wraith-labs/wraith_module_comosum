package radio

const (
	// Wraiths hit this endpoint on the c2 to register their existence.
	ROUTE_HEARTBEAT = "h"

	// The c2 hits this endpoint to access the Wraith SHM.
	ROUTE_EXCHANGE = "x"
)
