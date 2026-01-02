package novagate

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type fakeConn struct {
	mu sync.Mutex

	writeChunk int
	writeErr   error

	deadlines []time.Time
}

const errExpectedSetWriteDeadlineCalled = "expected SetWriteDeadline to be called"

func (c *fakeConn) Read(p []byte) (int, error)      { return 0, io.EOF }
func (c *fakeConn) Close() error                    { return nil }
func (c *fakeConn) LocalAddr() net.Addr             { return nil }
func (c *fakeConn) RemoteAddr() net.Addr            { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error { return nil }
func (c *fakeConn) SetDeadline(time.Time) error     { return nil }

func (c *fakeConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	if c.writeChunk <= 0 || c.writeChunk >= len(p) {
		return len(p), nil
	}
	return c.writeChunk, nil
}

func (c *fakeConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadlines = append(c.deadlines, t)
	return nil
}

func (c *fakeConn) lastWriteDeadline() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.deadlines) == 0 {
		return time.Time{}, false
	}
	return c.deadlines[len(c.deadlines)-1], true
}

func (c *fakeConn) sawNonZeroDeadline() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.deadlines {
		if !d.IsZero() {
			return true
		}
	}
	return false
}

func TestWriteAllClearsWriteDeadlineOnSuccess(t *testing.T) {
	c := &fakeConn{writeChunk: 2}
	data := []byte("hello")

	if err := writeAll(c, data, 50*time.Millisecond); err != nil {
		t.Fatalf("writeAll returned error: %v", err)
	}

	if !c.sawNonZeroDeadline() {
		t.Fatalf("expected SetWriteDeadline to be set to a non-zero time")
	}
	last, ok := c.lastWriteDeadline()
	if !ok {
		t.Fatalf(errExpectedSetWriteDeadlineCalled)
	}
	if !last.IsZero() {
		t.Fatalf("expected final write deadline to be cleared, got %v", last)
	}
}

func TestWriteAllClearsWriteDeadlineOnError(t *testing.T) {
	c := &fakeConn{writeErr: io.ErrClosedPipe}
	data := []byte("hello")

	if err := writeAll(c, data, 50*time.Millisecond); err == nil {
		t.Fatalf("expected error")
	}

	last, ok := c.lastWriteDeadline()
	if !ok {
		t.Fatalf(errExpectedSetWriteDeadlineCalled)
	}
	if !last.IsZero() {
		t.Fatalf("expected final write deadline to be cleared even on error, got %v", last)
	}
}

func TestWriteAllNoTimeoutKeepsDeadlineCleared(t *testing.T) {
	c := &fakeConn{}
	data := []byte("hello")

	if err := writeAll(c, data, 0); err != nil {
		t.Fatalf("writeAll returned error: %v", err)
	}

	if c.sawNonZeroDeadline() {
		t.Fatalf("did not expect a non-zero write deadline when timeout is disabled")
	}
	last, ok := c.lastWriteDeadline()
	if !ok {
		t.Fatalf(errExpectedSetWriteDeadlineCalled)
	}
	if !last.IsZero() {
		t.Fatalf("expected write deadline to be cleared, got %v", last)
	}
}
