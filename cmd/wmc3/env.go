package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	ENVIRONMENT_PREFIX = "WMC3_"

	ENV_DEBUG = ENVIRONMENT_PREFIX + "DEBUG"

	ENV_YGG_IDENTITY     = ENVIRONMENT_PREFIX + "YGG_IDENTITY"
	ENV_YGG_STATIC_PEERS = ENVIRONMENT_PREFIX + "YGG_STATIC_PEERS"
	ENV_YGG_LISTENERS    = ENVIRONMENT_PREFIX + "YGG_LISTENERS"

	ENV_NATS_ADMIN_USER = ENVIRONMENT_PREFIX + "NATS_ADMIN_USER"
	ENV_NATS_ADMIN_PASS = ENVIRONMENT_PREFIX + "NATS_ADMIN_PASS"
	ENV_NATS_LISTENER   = ENVIRONMENT_PREFIX + "NATS_LISTENER"
)

type conf struct {
	Debug bool

	YggIdentity    ed25519.PrivateKey
	YggStaticPeers []string
	YggListeners   []string

	NatsAdminUser string
	NatsAdminPass string
	NatsListener  string
}

func MakeConf() conf {
	envDebug := os.Getenv(ENV_DEBUG)
	if envDebug == "" {
		envDebug = "false"
	}
	parsedDebug, err := strconv.ParseBool(envDebug)
	if err != nil {
		panic(errors.Join(errors.New(fmt.Sprintf("could not parse value of env var %s", ENV_DEBUG)), err))
	}

	envYggIdentity := os.Getenv(ENV_YGG_IDENTITY)
	if envYggIdentity == "" {
		panic(errors.New("please define an yggdrasil identity"))
	}
	parsedYggIdentity, err := hex.DecodeString(envYggIdentity)
	if err != nil {
		panic(errors.Join(errors.New(fmt.Sprintf("could not parse value of env var %s", ENV_YGG_IDENTITY)), err))
	}

	var parsedYggStaticPeers []string
	envYggStaticPeers := os.Getenv(ENV_YGG_STATIC_PEERS)
	if envYggStaticPeers == "" {
		parsedYggStaticPeers = []string{}
	} else {
		parsedYggStaticPeers = strings.Split(envYggStaticPeers, ",")
	}
	for _, peer := range parsedYggStaticPeers {
		_, err := url.Parse(peer)
		if err != nil {
			panic(errors.Join(errors.New(fmt.Sprintf("could not parse value of env var %s, %s is invalid URL", ENV_YGG_IDENTITY, peer)), err))
		}
	}

	var parsedYggListeners []string
	envYggListeners := os.Getenv(ENV_YGG_LISTENERS)
	if envYggListeners == "" {
		parsedYggListeners = []string{}
	} else {
		parsedYggListeners = strings.Split(envYggListeners, ",")
	}
	for _, listener := range parsedYggListeners {
		_, err := url.Parse(listener)
		if err != nil {
			panic(errors.Join(errors.New(fmt.Sprintf("could not parse value of env var %s, %s is invalid URL", ENV_YGG_LISTENERS, listener)), err))
		}
	}

	envNatsAdminUser := os.Getenv(ENV_NATS_ADMIN_USER)
	envNatsAdminPass := os.Getenv(ENV_NATS_ADMIN_PASS)
	if envNatsAdminUser == "" || envNatsAdminPass == "" {
		panic(errors.New("please define an admin username and password"))
	}

	envNatsListener := os.Getenv(ENV_NATS_LISTENER)

	return conf{
		Debug:          parsedDebug,
		YggIdentity:    parsedYggIdentity,
		YggStaticPeers: parsedYggStaticPeers,
		YggListeners:   parsedYggListeners,
		NatsAdminUser:  envNatsAdminUser,
		NatsAdminPass:  envNatsAdminPass,
		NatsListener:   envNatsListener,
	}
}
