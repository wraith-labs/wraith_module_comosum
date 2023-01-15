package main

import (
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
func (s *state) Request(req proto.PacketReq) proto.PacketReq {
	reqTxId := uuid.NewString()
	req.TxId = reqTxId

	s.requestsMutex.Lock()
	defer s.requestsMutex.Unlock()

	s.requests[reqTxId] = request{
		requestTime: time.Now(),
		request:     req,
	}

	return req
}

// Save a response to a request.
func (s *state) Response(res proto.PacketRes) {
	s.requestsMutex.Lock()
	defer s.requestsMutex.Unlock()

	if req, ok := s.requests[res.TxId]; ok {
		req.responseTime = time.Now()
		req.response = res
		s.requests[res.TxId] = req
	}
}

// Expire timed-out entries in the state.
func (s *state) Prune() {}
