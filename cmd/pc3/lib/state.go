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

type client struct {
	ID      string `gorm:"primaryKey"`
	Address string `gorm:"index;not null;unique"`

	FirstHeartbeatTime time.Time             `gorm:"not null"`
	LastHeartbeatTime  time.Time             `gorm:"not null"`
	LastHeartbeat      proto.PacketHeartbeat `gorm:"not null;serializer:json;type:json"`
}

type request struct {
	TxId   string `gorm:"primaryKey"`
	Target string `gorm:"index;not null;unique"`

	RequestTime time.Time       `gorm:"not null"`
	Request     proto.PacketReq `gorm:"not null;serializer:json;type:json"`

	ResponseTime time.Time
	Response     proto.PacketRes `gorm:"serializer:json;type:json"`
}

func MkState() *state {
	//db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	db, err := gorm.Open(sqlite.Open("./test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to open memory db")
	}

	db.AutoMigrate(&client{}, &request{})

	return &state{
		db: db,
	}
}

func (s *state) ClientAppend(c *client) error {
	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "address"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_heartbeat_time", "last_heartbeat"}),
	}).Create(c)
	return result.Error
}

func (s *state) ClientDelete(c *client) error {
	result := s.db.Delete(c)
	return result.Error
}

func (s *state) ClientGet(id string) (client, error) {
	c := client{}
	result := s.db.Take(&c, id)
	return c, result.Error
}

func (s *state) ClientGetPage(offset, limit int) ([]client, error) {
	page := make([]client, limit)
	result := s.db.Order("first_heartbeat_time ASC").Find(&page)
	return page, result.Error
}

// Save/update a Wraith client entry.
func (s *state) Heartbeat(src string, hb proto.PacketHeartbeat) {
	s.ClientAppend(&client{
		ID:                 uuid.NewString(),
		Address:            src,
		FirstHeartbeatTime: time.Now(),
		LastHeartbeatTime:  time.Now(),
		LastHeartbeat:      hb,
	})
}

// Save a request and generate a TxId.
func (s *state) Request(dst string, req proto.PacketReq) proto.PacketReq {
	reqTxId := uuid.NewString()
	req.TxId = reqTxId

	s.db.Create(&request{
		TxId:        reqTxId,
		Target:      dst,
		RequestTime: time.Now(),
		Request:     req,
	})

	return req
}

// Save a response to a request.
func (s *state) Response(src string, res proto.PacketRes) error {
	req := request{}
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
		s.db.Where("last_heartbeat_time <= ?", time.Now().Add(-1*STATE_CLIENT_EXPIRY_DELAY)).Delete(&client{})
	}()

	// Clean up expired request-response pairs.
	go func() {
		defer wg.Done()
		s.db.Where("request_time <= ?", time.Now().Add(-1*STATE_REQUEST_EXPIRY_DELAY)).Delete(&request{})
	}()

	wg.Wait()
}
