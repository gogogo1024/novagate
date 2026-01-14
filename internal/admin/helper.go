package admin

import (
	"context"
	"time"
)

// NewContext creates a context with 5 second timeout
func NewContextTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
