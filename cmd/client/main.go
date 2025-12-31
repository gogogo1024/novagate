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
	addr := flag.String("addr", "127.0.0.1:9000", "server address")
	cmdHex := flag.String("cmd", "0x0001", "command id in hex, e.g. 0x0001")
	flagsHex := flag.String("flags", "0x00", "frame flags in hex, e.g. 0x04 for one-way")
	payloadStr := flag.String("payload", "ping", "payload string")
	reqID := flag.Uint64("id", 1, "request id")
	flag.Parse()

	cmdParsed, err := strconv.ParseUint(*cmdHex, 0, 16)
	if err != nil {
		panic(err)
	}
	flagsParsed, err := strconv.ParseUint(*flagsHex, 0, 8)
	if err != nil {
		panic(err)
	}
	flags := uint8(flagsParsed)

	conn, err := net.DialTimeout("tcp", *addr, 3*time.Second)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	req := &protocol.Message{
		Command:   uint16(cmdParsed),
		RequestID: *reqID,
		Payload:   []byte(*payloadStr),
	}
	msgBytes, err := protocol.EncodeMessage(req)
	if err != nil {
		panic(err)
	}
	frameFlags, frameBody, err := protocol.EncodeFrameBody(flags, msgBytes)
	if err != nil {
		panic(err)
	}
	frameBytes := protocol.Encode(&protocol.Frame{Flags: frameFlags, Body: frameBody})

	if _, err := conn.Write(frameBytes); err != nil {
		panic(err)
	}

	// One-way messages do not have a response by design.
	if flags&protocol.FlagOneWay != 0 {
		fmt.Printf("sent one-way: cmd=0x%04X request_id=%d payload=%q\n", req.Command, req.RequestID, string(req.Payload))
		return
	}

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 2048)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			frame, consumed, derr := protocol.Decode(buf)
			if derr != nil {
				panic(derr)
			}
			if frame != nil {
				respBody, err := protocol.DecodeFrameBody(frame)
				if err != nil {
					panic(err)
				}
				resp, err := protocol.DecodeMessage(respBody)
				if err != nil {
					panic(err)
				}
				fmt.Printf("resp: cmd=0x%04X request_id=%d payload=%q\n", resp.Command, resp.RequestID, string(resp.Payload))
				_ = consumed
				return
			}
		}
		if err != nil {
			// For non-one-way calls, timeout is a real error.
			panic(err)
		}
	}
}
