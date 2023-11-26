package proto

const (
	// Wraiths hit this endpoint on the c2 to fetch a registration token.
	ROUTE_REG = "r"

	// The c2 hits this endpoint to send data to a Wraith and receive a response.
	ROUTE_CMD = "c"
)
