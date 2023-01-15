package proto

// The structure of responses Wraiths running the pinecomms module
// make to pc3.
type PacketRes struct {
	// The main body of the response.
	Payload struct {
		// A map of all read cells and their contents.
		Read map[string]any

		// An array of all cells present in the shm if it was
		// requested.
		MemList []string
	}

	// A transaction ID allowing for mapping between requests
	// and responses. The TxId is opaque and can be any string
	// of any length.
	TxId string
}
