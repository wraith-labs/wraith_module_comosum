package lib

import (
	"context"
	"crypto/ed25519"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
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

type Client struct {
	ID      string `gorm:"primaryKey"`
	Address string `gorm:"index;not null;unique"`

	FirstHeartbeatTime time.Time             `gorm:"not null"`
	LastHeartbeatTime  time.Time             `gorm:"not null"`
	LastHeartbeat      proto.PacketHeartbeat `gorm:"not null;serializer:json;type:json"`
}

type Request struct {
	TxId   string `gorm:"primaryKey"`
	Target string `gorm:"index;not null;unique"`

	RequestTime time.Time      `gorm:"not null"`
	Request     proto.PacketRR `gorm:"not null;serializer:json;type:json"`

	ResponseTime time.Time
	Response     proto.PacketRR `gorm:"serializer:json;type:json"`
}
