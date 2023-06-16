package proto

// The structure of responses Wraiths running the pinecomms module
// make to pc3.
type PacketRes struct {
	// The main body of the response.
	Payload any

	// A transaction ID allowing for mapping between requests
	// and responses. The TxId is opaque and can be any string
	// of any length.
	TxId string
}
