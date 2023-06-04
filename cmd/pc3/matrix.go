package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func MatrixBotRunStartup(client *mautrix.Client, c Config) {
	// Make sure we're only ever in the admin room.
	rooms, err := client.JoinedRooms()
	if err != nil {
		panic(err)
	}
	for _, room := range rooms.JoinedRooms {
		if room.String() != c.adminRoom {
			client.LeaveRoom(room)
		}
	}
}

func MatrixBotEventHandlerSetUp(client *mautrix.Client, c Config) {
	syncer := client.Syncer.(*mautrix.DefaultSyncer)

	syncer.OnEventType(event.EventMessage, func(source mautrix.EventSource, evt *event.Event) {
		if evt.RoomID == id.RoomID(c.adminRoom) {
			client.SendMessageEvent(evt.RoomID, event.EventReaction, &map[string]any{
				"m.relates_to": map[string]any{
					"event_id": evt.ID,
					"key":      "🌊",
					"rel_type": "m.annotation",
				},
			})
		}
	})
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		if evt.GetStateKey() == client.UserID.String() && evt.Content.AsMember().Membership == event.MembershipInvite {
			if evt.RoomID == id.RoomID(c.adminRoom) {
				client.JoinRoomByID(evt.RoomID)
			} else {
				client.LeaveRoom(evt.RoomID)
			}
		}
	})
}

func MatrixBotInit(ctx context.Context, c Config, wg *sync.WaitGroup) *mautrix.Client {
	// Connect to Matrix homeserver.
	client, err := mautrix.NewClient(c.homeserver, "", "")
	if err != nil {
		panic(err)
	}

	pickleKey := make([]byte, 64)
	_, err = rand.Read(pickleKey)
	if err != nil {
		panic(err)
	}

	cryptoHelper, err := cryptohelper.NewCryptoHelper(client, pickleKey, "file::memory:")
	if err != nil {
		panic(err)
	}

	cryptoHelper.LoginAs = &mautrix.ReqLogin{
		Type:                     mautrix.AuthTypePassword,
		Identifier:               mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: c.username},
		InitialDeviceDisplayName: "WMP-" + fmt.Sprint(time.Now().Unix()),
		Password:                 c.password,
		StoreHomeserverURL:       true,
	}

	// Set the client crypto helper in order to automatically encrypt outgoing messages.
	err = cryptoHelper.Init()
	if err != nil {
		panic(err)
	}
	client.Crypto = cryptoHelper

	wg.Add(1)

	go func() {
		defer wg.Done()
		defer cryptoHelper.Close()

		for {
			err := client.SyncWithContext(ctx)
			if err == nil || errors.Is(err, context.Canceled) {
				break
			}
		}
	}()

	return client
}
