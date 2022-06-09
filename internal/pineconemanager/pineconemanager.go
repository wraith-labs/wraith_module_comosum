package pineconemanager

import (
	"context"
	"sync"
)

type pineconeManager struct {
	// sync.Once instances ensuring that each method is only executed once at a given time.
	startOnce   sync.Once
	stopOnce    sync.Once
	restartOnce sync.Once

	// A context (and related properties) which controls the lifetime of the pinecone manager.
	ctx       context.Context
	ctxCancel context.CancelFunc
	ctxLock   sync.RWMutex
}

func (pm *pineconeManager) Start() {
	// Only execute this once at a time.
	pm.startOnce.Do(func() {
		// Reset startOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.startOnce = sync.Once{}
		}(pm)

		// Set up the context to allow stopping the pinecone manager.
		pm.ctxLock.Lock()
		pm.ctx, pm.ctxCancel = context.WithCancel(context.Background())
		pm.ctxLock.Unlock()

		for {
			select {
			case <-pm.ctx.Done():
				break
			}
		}
	})
}

func (pm *pineconeManager) Stop() {
	// Only execute this once at a time.
	pm.stopOnce.Do(func() {
		// Reset stopOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.stopOnce = sync.Once{}
		}(pm)

		// Only actually do anything if the manager is running.
		// Note: This does not guarantee that the context is not cancelled
		// between the call to pm.IsRunning() and pm.ctxCancel(). A goroutine
		// could cancel the context after we check, which theoretically creates
		// a race condition. However, as a context CancelFunc is a no-op when
		// called multiple times, this is okay. The main reason for this check
		// is to prevent panics if the cancel func is nil which, it will be
		// before the manager's first run. As long as we know the manager
		// ran at some point (which this check guarantees), there won't be
		// issues.
		if pm.IsRunning() {
			pm.ctxCancel()
		}
	})
}

func (pm *pineconeManager) Restart() {
	// Only execute this once at a time.
	pm.restartOnce.Do(func() {
		// Reset restartOnce when this function exits.
		defer func(pm *pineconeManager) {
			pm.restartOnce = sync.Once{}
		}(pm)

		pm.Stop()
		pm.Start()
	})
}

func (pm *pineconeManager) IsRunning() bool {
	// Make sure the context isn't modified while we're checking it.
	defer pm.ctxLock.RUnlock()
	pm.ctxLock.RLock()

	// If the context is nil, we're definitely not running.
	if pm.ctx == nil {
		return false
	}

	// If the context is not nil, we need to check if context.Err()
	// is nil to determine if the pm is running.
	if pm.ctx.Err() != nil {
		return false
	}

	return true
}

var initonce sync.Once
var pineconeManagerInstance *pineconeManager = nil

func GetInstance() *pineconeManager {
	// Create and initialise an instance of pineconeManager only once.
	initonce.Do(func() {
		pineconeManagerInstance = &pineconeManager{}
	})

	return pineconeManagerInstance
}
