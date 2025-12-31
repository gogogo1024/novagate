package novagate

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogogo1024/novagate/protocol"
)

// Handler handles a decoded protocol message.
// Returning (nil, nil) means no response.
type Handler func(context.Context, *protocol.Message) (*protocol.Message, error)

// Router is the default in-process command router.
// It is safe for concurrent use.
type Router struct {
	mu       sync.RWMutex
	handlers map[uint16]Handler
}

func NewRouter() *Router {
	return &Router{handlers: make(map[uint16]Handler)}
}

func (r *Router) Register(cmd uint16, h Handler) {
	r.mu.Lock()
	r.handlers[cmd] = h
	r.mu.Unlock()
}

func (r *Router) Dispatch(ctx context.Context, m *protocol.Message) (*protocol.Message, error) {
	r.mu.RLock()
	h := r.handlers[m.Command]
	r.mu.RUnlock()
	if h == nil {
		return nil, fmt.Errorf("unknown command: 0x%04X", m.Command)
	}
	return h(ctx, m)
}
