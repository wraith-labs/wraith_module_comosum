package lib

import (
	"context"
	"crypto/ed25519"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/internal/proto"
	"dev.l1qu1d.net/wraith-labs/wraith_module_comosum/internal/radio"
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
	Address string `gorm:"primaryKey"`

	FirstHeartbeatTime time.Time             `gorm:"not null"`
	LastHeartbeatTime  time.Time             `gorm:"not null"`
	LastHeartbeat      proto.PacketHeartbeat `gorm:"not null;serializer:json;type:json"`
}

type Request struct {
	TxId   string `gorm:"primaryKey"`
	Target string `gorm:"index;not null"`

	RequestTime time.Time      `gorm:"not null"`
	Request     proto.PacketRR `gorm:"not null;serializer:json;type:json"`

	ResponseTime time.Time
	Response     proto.PacketRR `gorm:"serializer:json;type:json"`
}
