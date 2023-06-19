package lib

import (
	"context"
	"crypto/ed25519"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/radio"
	"maunium.net/go/mautrix"
)

type CommandContext struct {
	Context    context.Context
	Config     *Config
	Client     *mautrix.Client
	Radio      *radio.Radio
	State      *state
	OwnPrivKey ed25519.PrivateKey
}
