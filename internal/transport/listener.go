package transport

import "sync"

// Listener processes incoming messages from UDS connections.
type Listener struct {
	mu        sync.RWMutex
	callbacks []func(data []byte)
}

// NewListener creates a message listener.
func NewListener() *Listener {
	return &Listener{}
}

// OnMessage registers a callback for incoming payloads.
func (l *Listener) OnMessage(fn func(data []byte)) {
	if fn == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.callbacks = append(l.callbacks, fn)
}

// Handle invokes all registered callbacks with the incoming payload.
func (l *Listener) Handle(data []byte) {
	l.mu.RLock()
	callbacks := append([]func(data []byte){}, l.callbacks...)
	l.mu.RUnlock()

	for _, callback := range callbacks {
		callback(data)
	}
}
