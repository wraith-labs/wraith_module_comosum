package lib

import (
	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/radio"
	"maunium.net/go/mautrix"
)

type CommandContext struct {
	Config *Config
	Client *mautrix.Client
	Radio  *radio.Radio
}
