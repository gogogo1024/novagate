package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gogogo1024/novagate/protocol"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

type clientConfig struct {
	addr     string
	cmdHex   string
	flagsHex string
	payload  string
	reqID    uint64
}

func run() error {
	cfg := parseFlags()

	cmd, flags, err := parseCommandAndFlags(cfg.cmdHex, cfg.flagsHex)
	if err != nil {
		return err
	}

	conn, err := dial(cfg.addr, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := &protocol.Message{
		Command:   cmd,
		RequestID: cfg.reqID,
		Payload:   []byte(cfg.payload),
	}
	if err := sendRequest(conn, flags, req); err != nil {
		return err
	}

	// One-way messages do not have a response by design.
	if flags&protocol.FlagOneWay != 0 {
		printSentOneWay(req)
		return nil
	}

	resp, err := readResponse(conn, 3*time.Second)
	if err != nil {
		return err
	}
	printResponse(resp)
	return nil
}

func parseFlags() clientConfig {
	addr := flag.String("addr", "127.0.0.1:9000", "server address")
	cmdHex := flag.String("cmd", "0x0001", "command id in hex, e.g. 0x0001")
	flagsHex := flag.String("flags", "0x00", "frame flags in hex, e.g. 0x04 for one-way")
	payloadStr := flag.String("payload", "ping", "payload string")
	reqID := flag.Uint64("id", 1, "request id")
	flag.Parse()

	return clientConfig{
		addr:     *addr,
		cmdHex:   *cmdHex,
		flagsHex: *flagsHex,
		payload:  *payloadStr,
		reqID:    *reqID,
	}
}

func parseCommandAndFlags(cmdHex string, flagsHex string) (uint16, uint8, error) {
	cmdParsed, err := strconv.ParseUint(cmdHex, 0, 16)
	if err != nil {
		return 0, 0, err
	}
	flagsParsed, err := strconv.ParseUint(flagsHex, 0, 8)
	if err != nil {
		return 0, 0, err
	}
	return uint16(cmdParsed), uint8(flagsParsed), nil
}

func dial(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp", addr, timeout)
}

func sendRequest(conn net.Conn, flags uint8, req *protocol.Message) error {
	msgBytes, err := protocol.EncodeMessage(req)
	if err != nil {
		return err
	}
	frameFlags, frameBody, err := protocol.EncodeFrameBody(flags, msgBytes)
	if err != nil {
		return err
	}
	frameBytes := protocol.Encode(&protocol.Frame{Flags: frameFlags, Body: frameBody})

	_, err = conn.Write(frameBytes)
	return err
}

func readResponse(conn net.Conn, timeout time.Duration) (*protocol.Message, error) {
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 2048)
	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			frame, _, derr := protocol.Decode(buf)
			if derr != nil {
				return nil, derr
			}
			if frame != nil {
				respBody, err := protocol.DecodeFrameBody(frame)
				if err != nil {
					return nil, err
				}
				return protocol.DecodeMessage(respBody)
			}
		}
		if err != nil {
			// For non-one-way calls, timeout is a real error.
			return nil, err
		}
	}
}

func printSentOneWay(req *protocol.Message) {
	fmt.Printf("sent one-way: cmd=0x%04X request_id=%d payload=%q\n", req.Command, req.RequestID, string(req.Payload))
}

func printResponse(resp *protocol.Message) {
	fmt.Printf("resp: cmd=0x%04X request_id=%d payload=%q\n", resp.Command, resp.RequestID, string(resp.Payload))
}
