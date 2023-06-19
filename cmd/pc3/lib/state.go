package lib

import (
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

type Client struct {
	ID      string `gorm:"primaryKey"`
	Address string `gorm:"index;not null;unique"`

	FirstHeartbeatTime time.Time             `gorm:"not null"`
	LastHeartbeatTime  time.Time             `gorm:"not null"`
	LastHeartbeat      proto.PacketHeartbeat `gorm:"not null;serializer:json;type:json"`
}

type Request struct {
	TxId   string `gorm:"primaryKey"`
	Target string `gorm:"index;not null;unique"`

	RequestTime time.Time      `gorm:"not null"`
	Request     proto.PacketRR `gorm:"not null;serializer:json;type:json"`

	ResponseTime time.Time
	Response     proto.PacketRR `gorm:"serializer:json;type:json"`
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

func (s *state) ClientGet(id string) (Client, error) {
	c := Client{}
	result := s.db.Take(&c, id)
	return c, result.Error
}

func (s *state) ClientGetPage(offset, limit int) ([]Client, error) {
	clients := make([]Client, limit)
	result := s.db.Order("first_heartbeat_time ASC").Offset(offset).Limit(limit).Find(&clients)
	return clients, result.Error
}

func (s *state) ClientGetAll() ([]Client, error) {
	c := []Client{}
	result := s.db.Find(&c)
	return c, result.Error
}

func (s *state) ClientCount() (int64, error) {
	var count int64
	result := s.db.Model(&Client{}).Count(&count)
	return count, result.Error
}

// Save/update a Wraith client entry.
func (s *state) Heartbeat(src string, hb proto.PacketHeartbeat) {
	s.ClientAppend(&Client{
		ID:                 uuid.NewString(),
		Address:            src,
		FirstHeartbeatTime: time.Now(),
		LastHeartbeatTime:  time.Now(),
		LastHeartbeat:      hb,
	})
}

// Save a request and generate a TxId.
func (s *state) Request(dst string, req proto.PacketRR) proto.PacketRR {
	reqTxId := uuid.NewString()
	req.TxId = reqTxId

	s.db.Create(&Request{
		TxId:        reqTxId,
		Target:      dst,
		RequestTime: time.Now(),
		Request:     req,
	})

	return req
}

// Save a response to a request.
func (s *state) Response(src string, res proto.PacketRR) error {
	req := Request{}
	result := s.db.First(&req, res.TxId)
	if result.Error == nil && src == req.Target && req.ResponseTime.IsZero() {
		req.ResponseTime = time.Now()
		req.Response = res
		s.db.Save(req)
	}
	return result.Error
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
