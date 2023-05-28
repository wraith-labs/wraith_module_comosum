package radio

/*

Stolen from https://github.com/matrix-org/pinecone/blob/c05f24e907e9eb0f84384bfa226dad174f5ea2ad/util/websocket.go and hence:

// Copyright 2021 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

Modifications:
- Change wrapWebSocketConn and webSocketConn to private

*/

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

func wrapWebSocketConn(c *websocket.Conn) *webSocketConn {
	return &webSocketConn{c: c}
}

type webSocketConn struct {
	r io.Reader
	c *websocket.Conn
}

func (c *webSocketConn) Write(p []byte) (int, error) {
	err := c.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *webSocketConn) Read(p []byte) (int, error) {
	for {
		if c.r == nil {
			// Advance to next message.
			var err error
			_, c.r, err = c.c.NextReader()
			if err != nil {
				return 0, err
			}
		}
		n, err := c.r.Read(p)
		if err == io.EOF {
			// At end of message.
			c.r = nil
			if n > 0 {
				return n, nil
			} else {
				// No data read, continue to next message.
				continue
			}
		}
		return n, err
	}
}

func (c *webSocketConn) Close() error {
	return c.c.Close()
}

func (c *webSocketConn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *webSocketConn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

func (c *webSocketConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

func (c *webSocketConn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *webSocketConn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}
