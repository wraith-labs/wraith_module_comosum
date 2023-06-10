package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
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
			client.ForgetRoom(room)
		}
	}
}

func MatrixBotEventHandlerSetUp(client *mautrix.Client, c Config) {
	syncer := client.Syncer.(*mautrix.DefaultSyncer)

	// Messages.
	syncer.OnEventType(event.EventMessage, func(source mautrix.EventSource, evt *event.Event) {
		if evt.RoomID == id.RoomID(c.adminRoom) {
			// Mark any messages in the admin room as read.
			client.SendReceipt(evt.RoomID, evt.ID, event.ReceiptTypeRead, nil)

			if message, ok := evt.Content.Parsed.(*event.MessageEventContent); ok {
				if command := strings.TrimPrefix(message.Body, "!wmp "); command != message.Body {
					// If the message starts with the command prefix, start
					// typing to indicate that we're processing the message.
					defer client.UserTyping(evt.RoomID, false, time.Microsecond*1)
					client.UserTyping(evt.RoomID, true, time.Minute*1)

					if replyText, err := ExecCmd(command); replyText != nil {
						// If there is a reply, send it.
						reply := format.RenderMarkdown(*replyText, true, true)
						reply.SetReply(evt)
						client.SendMessageEvent(evt.RoomID, event.EventMessage, reply)
					} else if err != nil {
						// If there is no reply but there is an error, react with nack.
						client.SendReaction(evt.RoomID, evt.ID, "❌")
					} else {
						// Otherwise, react with ack.
						client.SendReaction(evt.RoomID, evt.ID, "✅")
					}
				}
			}
		}
	})

	// Invites.
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		if evt.GetStateKey() == client.UserID.String() && evt.Content.AsMember().Membership == event.MembershipInvite {
			if evt.RoomID == id.RoomID(c.adminRoom) {
				client.JoinRoomByID(evt.RoomID)
			} else {
				client.LeaveRoom(evt.RoomID)
				client.ForgetRoom(evt.RoomID)
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

	// Set up the client crypto helper in order to automatically encrypt outgoing messages.
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
