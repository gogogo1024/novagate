package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogogo1024/novagate"
	"github.com/gogogo1024/novagate/internal/dispatcher"
	"github.com/gogogo1024/novagate/internal/service"
	"github.com/gogogo1024/novagate/protocol"
)

func setup(r *novagate.Router) error {
	// Command table (docs/protocol.md examples)
	protocol.RegisterFullMethodCommand("NovaService.Ping", protocol.CmdPing)
	protocol.RegisterFullMethodCommand("UserService.Login", protocol.CmdUserLogin)
	protocol.RegisterFullMethodCommand("OrderService.Create", protocol.CmdOrderCreate)
	protocol.SetStrictCommandMapping(true)

	// Business dispatcher handlers
	service.RegisterHandlers()

	// Protocol router handlers (bridge to dispatcher)
	bridge := func(cmd uint16) {
		r.Register(cmd, novagate.BridgeProtocolHandler(cmd, func(ctx context.Context, payload []byte) ([]byte, error) {
			return dispatcher.Dispatch(ctx, cmd, payload)
		}))
	}
	bridge(protocol.CmdPing)
	bridge(protocol.CmdUserLogin)
	bridge(protocol.CmdOrderCreate)

	return nil
}

func main() {
	// Defensive: in some environments `go test ./...` may execute command mains.
	// Avoid starting a long-running listener from a test binary.
	if strings.HasSuffix(filepath.Base(os.Args[0]), ".test") {
		return
	}

	addr := flag.String("addr", ":9000", "listen address")
	flag.Parse()

	log.Printf("novagate listening on %s", *addr)
	if err := novagate.ListenAndServe(*addr, setup); err != nil {
		log.Fatal(err)
	}
}
