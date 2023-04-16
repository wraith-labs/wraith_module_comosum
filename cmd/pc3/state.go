package main

import (
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"github.com/google/uuid"
)

func MkState() *state {
	s := state{
		clients: clientList{
			clients: map[string]*client{},
		},
		requests: map[string]request{},
	}
	return &s
}

type client struct {
	Address           string                `json:"address"`
	LastHeartbeatTime time.Time             `json:"lastHeartbeatTime"`
	LastHeartbeat     proto.PacketHeartbeat `json:"lastHeartbeat"`

	prev *client
	next *client
}

// This data structure allows for storage of clients while allowing for efficient ordering
// and therefore pagination, deletion and addition of clients, and accessing a client by ID.
type clientList struct {
	head    *client
	tail    *client
	clients map[string]*client
}

func (l *clientList) Append(id string, c client) {
	c.prev, c.next = nil, nil
	l.clients[id] = &c
	if l.head == nil {
		l.head = &c
	}
	if l.tail != nil {
		l.tail.next = &c
		c.prev = l.tail
	}
	l.tail = &c
}

func (l *clientList) Delete(id string) {
	c, ok := l.clients[id]
	if !ok {
		return
	}

	if c.prev == nil {
		// This is the first element. Make the next one first.
		l.head = c.next
	} else {
		c.prev.next = c.next
	}

	if c.next == nil {
		// This is the last element. Make the previous one last.
		l.tail = c.prev
	} else {
		c.next.prev = c.prev
	}

	delete(l.clients, id)
}

func (l *clientList) Get(id string) (*client, bool) {
	c, ok := l.clients[id]
	return c, ok
}

func (l *clientList) GetPage(offset, limit int) []*client {
	if offset > len(l.clients) {
		return []*client{}
	}

	// If the remainder of the client list after the offset is
	// smaller than the limit, reduce the limit to the size of that
	// remainder to avoid nulls in the returned data.
	if maxLimit := len(l.clients) - offset; maxLimit < limit {
		limit = maxLimit
	}

	page := make([]*client, limit)
	current := l.head
	for i := 0; i < offset+limit; i++ {
		if current == nil {
			break
		}

		if i >= offset && i < offset+limit {
			page[i-offset] = current
		}

		current = current.next
	}
	return page
}

type request struct {
	target string

	requestTime time.Time
	request     proto.PacketReq

	responseTime time.Time
	response     proto.PacketRes
}

type state struct {
	// List of "connected" Wraith clients.
	clients      clientList
	clientsMutex sync.RWMutex

	// List of request/response pairs.
	requests      map[string]request
	requestsMutex sync.RWMutex
}

// Save/update a Wraith client entry.
func (s *state) Heartbeat(src string, hb proto.PacketHeartbeat) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	s.clients.Append(src, client{
		Address:           src,
		LastHeartbeatTime: time.Now(),
		LastHeartbeat:     hb,
	})
}

// Save a request and generate a TxId.
func (s *state) Request(dst string, req proto.PacketReq) proto.PacketReq {
	reqTxId := uuid.NewString()
	req.TxId = reqTxId

	s.requestsMutex.Lock()
	defer s.requestsMutex.Unlock()

	s.requests[reqTxId] = request{
		target:      dst,
		requestTime: time.Now(),
		request:     req,
	}

	return req
}

// Save a response to a request.
func (s *state) Response(src string, res proto.PacketRes) {
	s.requestsMutex.Lock()
	defer s.requestsMutex.Unlock()

	if req, ok := s.requests[res.TxId]; ok && src == req.target && req.responseTime.IsZero() {
		req.responseTime = time.Now()
		req.response = res
		s.requests[res.TxId] = req
	}
}

// Expire timed-out entries in the state.
func (s *state) Prune() {
	wg := sync.WaitGroup{}
	wg.Add(2)

	// Clean up expired client heartbeats.
	go func() {
		defer wg.Done()
		s.clientsMutex.Lock()
		defer s.clientsMutex.Unlock()

		for id, c := range s.clients.clients {
			if time.Since(c.LastHeartbeatTime) > proto.HEARTBEAT_MARK_DEAD_DELAY {
				s.clients.Delete(id)
			}
		}
	}()

	// Clean up expired request-response pairs.
	go func() {
		defer wg.Done()
		s.requestsMutex.Lock()
		defer s.requestsMutex.Unlock()

		for id, r := range s.requests {
			if time.Since(r.requestTime) > STATE_REQUEST_EXPIRY_DELAY {
				delete(s.requests, id)
			}
		}
	}()

	wg.Wait()
}

func (s *state) GetClients(offset, limit int) []*client {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	return s.clients.GetPage(offset, limit)
}
