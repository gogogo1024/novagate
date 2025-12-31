package dispatcher

import (
	"context"
	"fmt"
)

type Handler func(context.Context, []byte) ([]byte, error)

var handlers = map[uint16]Handler{}

func Register(cmd uint16, h Handler) {
	handlers[cmd] = h
}

func Dispatch(ctx context.Context, cmd uint16, payload []byte) ([]byte, error) {
	h, ok := handlers[cmd]
	if !ok {
		return nil, fmt.Errorf("no dispatcher handler for command 0x%04X", cmd)
	}
	return h(ctx, payload)
}
