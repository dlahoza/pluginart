package protocol

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func BenchmarkHandleConnServer(b *testing.B) {
	for _, size := range []int{10, 1000, 10000} {
		b.Run(payloadName(size), func(b *testing.B) {
			payload := bytes.Repeat([]byte("x"), size)
			c := newServerBenchmarkConn(b.N, payload)
			handler := handlerFunc(func(_ context.Context, payload []byte) ([]byte, error) {
				return payload, nil
			})

			b.ReportAllocs()
			b.SetBytes(int64(size))
			b.ResetTimer()
			handleConn(c, handler, "hash")
			b.StopTimer()
			if c.writeErr != nil {
				b.Fatalf("write response: %v", c.writeErr)
			}
		})
	}
}

func buildFrame(dst []byte, msgType MsgType, payload []byte) []byte {
	size := headerSize + len(payload)
	if cap(dst) < size {
		dst = make([]byte, size)
	}
	dst = dst[:size]
	copy(dst[:4], magic[:])
	binary.LittleEndian.PutUint32(dst[4:8], uint32(len(payload))) //nolint:gosec // benchmark payloads are small and fixed.
	dst[8] = byte(msgType)
	copy(dst[headerSize:], payload)
	return dst
}

func payloadName(size int) string {
	switch size {
	case 10:
		return "payload_10_bytes"
	case 1000:
		return "payload_1000_bytes"
	case 10000:
		return "payload_10000_bytes"
	default:
		return "payload"
	}
}

type serverBenchmarkConn struct {
	handshake []byte
	payload   []byte
	callsLeft int
	frame     []byte
	offset    int
	writeErr  error
}

func newServerBenchmarkConn(calls int, payload []byte) *serverBenchmarkConn {
	return &serverBenchmarkConn{
		handshake: buildHandshakeRequest("hash", "bench"),
		payload:   payload,
		callsLeft: calls,
	}
}

func (c *serverBenchmarkConn) Read(p []byte) (int, error) {
	for c.offset >= len(c.frame) {
		if c.handshake != nil {
			c.frame = buildFrame(c.frame, MsgHandshakeRequest, c.handshake)
			c.handshake = nil
			c.offset = 0
			break
		}
		if c.callsLeft <= 0 {
			return 0, io.EOF
		}
		c.frame = buildFrame(c.frame, MsgCallRequest, c.payload)
		c.callsLeft--
		c.offset = 0
	}
	n := copy(p, c.frame[c.offset:])
	c.offset += n
	return n, nil
}

func (c *serverBenchmarkConn) Write(p []byte) (int, error) {
	return len(p), c.writeErr
}

func (c *serverBenchmarkConn) Close() error {
	return nil
}

func (c *serverBenchmarkConn) LocalAddr() net.Addr {
	return dummyAddr("scripted")
}

func (c *serverBenchmarkConn) RemoteAddr() net.Addr {
	return dummyAddr("scripted")
}

func (c *serverBenchmarkConn) SetDeadline(time.Time) error {
	return nil
}

func (c *serverBenchmarkConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *serverBenchmarkConn) SetWriteDeadline(time.Time) error {
	return nil
}
