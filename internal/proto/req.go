package proto

// The structure of requests pc3 makes to Wraiths running the
// pinecomms module.
type Req struct {
	// The actual payload which tells the module what to do.
	Payload struct {
		// Which shm fields should be read and returned.
		Read []string

		// Which shm fields should be written to and the
		// values that should be written.
		Write map[string]interface{}

		// Whether to return a list of all active memory cells
		// in the response. Runs after Req.Payload.Write.
		ListMem bool
	}

	// Conditions which must be satisfied for Wraith to consider
	// the payload. If the conditions are not met, the payload
	// is dropped.
	Conditions struct{}
}
