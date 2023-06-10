package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/cmd/pc3/lib"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

func MatrixBotRunStartup(client *mautrix.Client, c lib.Config) {
	// Make sure we're only ever in the admin room.
	rooms, err := client.JoinedRooms()
	if err != nil {
		panic(err)
	}
	for _, room := range rooms.JoinedRooms {
		if room.String() != c.AdminRoom {
			client.LeaveRoom(room)
			client.ForgetRoom(room)
		}
	}
}

func MatrixBotEventHandlerSetUp(ctx lib.CommandContext) {
	syncer := ctx.Client.Syncer.(*mautrix.DefaultSyncer)

	// Messages.
	syncer.OnEventType(event.EventMessage, func(source mautrix.EventSource, evt *event.Event) {
		if evt.RoomID == id.RoomID(ctx.Config.AdminRoom) {
			// Mark any messages in the admin room as read.
			ctx.Client.SendReceipt(evt.RoomID, evt.ID, event.ReceiptTypeRead, nil)

			if message, ok := evt.Content.Parsed.(*event.MessageEventContent); ok {
				if command := strings.TrimPrefix(message.Body, "!wmp "); command != message.Body {
					// If the message starts with the command prefix, start
					// typing to indicate that we're processing the message.
					defer ctx.Client.UserTyping(evt.RoomID, false, time.Microsecond*1)
					ctx.Client.UserTyping(evt.RoomID, true, time.Minute*1)

					replyText, err := ExecCmd(ctx, command)
					if err == nil {
						ctx.Client.SendReaction(evt.RoomID, evt.ID, "✅")
					} else {
						ctx.Client.SendReaction(evt.RoomID, evt.ID, "❌")

						errReply := format.RenderMarkdown(err.Error(), true, true)
						errReply.SetReply(evt)
						ctx.Client.SendMessageEvent(evt.RoomID, event.EventMessage, errReply)
					}
					if replyText != "" {
						reply := format.RenderMarkdown(replyText, true, true)
						reply.SetReply(evt)
						ctx.Client.SendMessageEvent(evt.RoomID, event.EventMessage, reply)
					}
				}
			}
		}
	})

	// Invites.
	syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
		if evt.GetStateKey() == ctx.Client.UserID.String() && evt.Content.AsMember().Membership == event.MembershipInvite {
			if evt.RoomID == id.RoomID(ctx.Config.AdminRoom) {
				ctx.Client.JoinRoomByID(evt.RoomID)
			} else {
				ctx.Client.LeaveRoom(evt.RoomID)
				ctx.Client.ForgetRoom(evt.RoomID)
			}
		}
	})
}

func MatrixBotInit(ctx context.Context, c lib.Config, wg *sync.WaitGroup) *mautrix.Client {
	// Connect to Matrix homeserver.
	client, err := mautrix.NewClient(c.Homeserver, "", "")
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
		Identifier:               mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: c.Username},
		InitialDeviceDisplayName: "WMP-" + fmt.Sprint(time.Now().Unix()),
		Password:                 c.Password,
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
