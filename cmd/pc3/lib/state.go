package lib

import (
	"fmt"
	"sync"
	"time"

	"dev.l1qu1d.net/wraith-labs/wraith-module-pinecomms/internal/proto"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type state struct {
	db *gorm.DB
}

func MkState() *state {
	db, err := gorm.Open(sqlite.Open("./test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to open memory db")
	}

	db.AutoMigrate(&Client{}, &Request{})

	return &state{
		db: db,
	}
}

func (s *state) ClientAppend(c *Client) error {
	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_heartbeat_time", "last_heartbeat"}),
	}).Create(c)
	return result.Error
}

func (s *state) ClientDelete(c *Client) error {
	result := s.db.Delete(c)
	return result.Error
}

func (s *state) ClientsGet(addresses []string) ([]Client, error) {
	var clients []Client
	result := s.db.Find(&clients, addresses)
	return clients, result.Error
}

func (s *state) ClientsGetExcept(addresses []string) ([]Client, error) {
	var clients []Client
	result := s.db.Not(addresses).Find(&clients)
	return clients, result.Error
}

func (s *state) ClientGetPage(offset, limit int) ([]Client, error) {
	var clients []Client
	result := s.db.Order("first_heartbeat_time ASC").Offset(offset).Limit(limit).Find(&clients)
	return clients, result.Error
}

func (s *state) ClientGetAll() ([]Client, error) {
	var clients []Client
	result := s.db.Find(&clients)
	return clients, result.Error
}

func (s *state) ClientCount() (int64, error) {
	var count int64
	result := s.db.Model(&Client{}).Count(&count)
	return count, result.Error
}

// Save/update a Wraith client entry.
func (s *state) Heartbeat(src string, hb proto.PacketHeartbeat) {
	s.ClientAppend(&Client{
		Address:            src,
		FirstHeartbeatTime: time.Now(),
		LastHeartbeatTime:  time.Now(),
		LastHeartbeat:      hb,
	})
}

// Save a request and generate a TxId.
func (s *state) Request(dst string, req proto.PacketRR) (proto.PacketRR, error) {
	reqTxId := uuid.NewString()
	req.TxId = reqTxId

	result := s.db.Create(&Request{
		TxId:        reqTxId,
		Target:      dst,
		RequestTime: time.Now(),
		Request:     req,
	})

	return req, result.Error
}

// Save a response to a request.
func (s *state) Response(src string, res proto.PacketRR) error {
	req := Request{}
	result := s.db.Take(&req, "tx_id = ?", res.TxId)
	if result.Error == nil && src == req.Target && req.ResponseTime.IsZero() {
		req.ResponseTime = time.Now()
		req.Response = res
		result = s.db.Save(req)
	}
	return result.Error
}

func (s *state) AwaitResponse(txId string, timeout time.Duration) (*proto.PacketRR, error) {
	startTime := time.Now()
	for {
		req := Request{}
		result := s.db.Take(&req, "tx_id = ?", txId)

		if result.Error != nil {
			return nil, result.Error
		} else if !req.ResponseTime.IsZero() {
			return &req.Response, nil
		}

		if time.Since(startTime) > timeout {
			return nil, fmt.Errorf("timeout waiting for response to request `%s` after %s", txId, time.Since(startTime).String())
		}

		<-time.After(time.Second)
	}
}

// Expire timed-out entries in the state.
func (s *state) Prune() {
	wg := sync.WaitGroup{}
	wg.Add(2)

	// Clean up expired client heartbeats.
	go func() {
		defer wg.Done()
		s.db.Where("last_heartbeat_time <= ?", time.Now().Add(-1*STATE_CLIENT_EXPIRY_DELAY)).Delete(&Client{})
	}()

	// Clean up expired request-response pairs.
	go func() {
		defer wg.Done()
		s.db.Where("request_time <= ?", time.Now().Add(-1*STATE_REQUEST_EXPIRY_DELAY)).Delete(&Request{})
	}()

	wg.Wait()
}
