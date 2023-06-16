package proto

// The structure of requests pc3 makes to Wraiths running the
// pinecomms module.
type PacketReq struct {
	// The actual payload which tells the module what to do.
	Payload string

	// A transaction ID allowing for mapping between requests
	// and responses. The TxId is opaque and can be any string
	// of any length.
	TxId string
}
