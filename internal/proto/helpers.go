package proto

import (
	"crypto/ed25519"
	"crypto/sha512"
	"errors"

	"github.com/fxamacker/cbor/v2"
)

type packet interface {
	PacketRegReq | PacketRegRes |
	PacketCmdReq | PacketCmdRes
}

// Converts a packet into a byte array ready for transmission.
func Marshal[P packet](p *P, signingKey ed25519.PrivateKey) ([]byte, error) {
	// Get the CBOR representation of the data.
	dataBytes, err := cbor.Marshal(p)
	if err != nil {
		return dataBytes, err
	}

	// Calculate the checksum of the data for the signature.
	checksum := sha512.Sum384(dataBytes)

	// Create the signature for verification purposes.
	signatureBytes := ed25519.Sign(signingKey, checksum[:])

	// Return a byte array of the signature followed by the data.
	return append(signatureBytes, dataBytes...), nil
}

// Converts a byte array into a packet so that it can be processed.
func Unmarshal[P packet](p *P, verificationKey ed25519.PublicKey, data []byte) error {
	// Make sure the data is correctly formatted (at least 64 bytes)
	if len(data) < 64 {
		return errors.New("provided data was too short")
	}

	// Split the byte array into the signature and data parts.
	signatureBytes := data[0:64]
	dataBytes := data[64:]

	// Calculate the checksum of the data for verification.
	checksum := sha512.Sum384(dataBytes)

	// Verify the signature.
	verified := ed25519.Verify(verificationKey, checksum[:], signatureBytes)
	if !verified {
		return errors.New("data failed signature verification")
	}

	// If verification was successful, unmarshal the data into the
	// current struct and return whether this was successful.
	return cbor.Unmarshal(dataBytes, p)
}
