package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func main() {
	pubkey, prvkey, err := ed25519.GenerateKey(rand.Reader)

	if err != nil {
		panic(err)
	}

	fmt.Printf(
		"PUBLIC KEY: %s\nPRIVATE KEY: %s\n",
		hex.EncodeToString(pubkey),
		hex.EncodeToString(prvkey),
	)
}
