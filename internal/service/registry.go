package service

import (
	"context"

	"github.com/gogogo1024/novagate/internal/dispatcher"
	"github.com/gogogo1024/novagate/protocol"
)

// RegisterHandlers wires example protocol commands to service handlers.
//
// This keeps main.go thin and allows you to later swap the implementation
// (e.g. call Kitex RPC) without changing the protocol/transport stack.
func RegisterHandlers() {
	dispatcher.Register(protocol.CmdPing, func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("pong"), nil
	})

	dispatcher.Register(protocol.CmdUserLogin, func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	})

	dispatcher.Register(protocol.CmdOrderCreate, func(ctx context.Context, payload []byte) ([]byte, error) {
		return []byte("ok"), nil
	})
}
