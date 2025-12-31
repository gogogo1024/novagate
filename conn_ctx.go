package novagate

import (
	"sync/atomic"
	"time"
)

type ConnContext struct {
	bufferUsed int64
	maxBuffer  int64
	tokens     int64
	lastRefill int64
	rate       int64
	burst      int64
}

func NewConnContext() *ConnContext {
	now := time.Now().UnixNano()
	return &ConnContext{
		maxBuffer:  256 * 1024,
		tokens:     100,
		lastRefill: now,
		rate:       100,
		burst:      200,
	}
}

func (c *ConnContext) Reserve(n int) bool {
	used := atomic.AddInt64(&c.bufferUsed, int64(n))
	return used <= c.maxBuffer
}

func (c *ConnContext) Release(n int) {
	atomic.AddInt64(&c.bufferUsed, -int64(n))
}

func (c *ConnContext) Allow() bool {
	now := time.Now().UnixNano()
	elapsed := now - c.lastRefill

	add := elapsed * c.rate / int64(time.Second)
	if add > 0 {
		newTokens := c.tokens + add
		if newTokens > c.burst {
			newTokens = c.burst
		}
		c.tokens = newTokens
		c.lastRefill = now
	}

	if c.tokens <= 0 {
		return false
	}

	c.tokens--
	return true
}
