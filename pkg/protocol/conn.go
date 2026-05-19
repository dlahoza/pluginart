package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	maxFrameSize = 4 * 1024 * 1024 // 4 MiB
	headerSize   = 9               // 4 magic + 4 length + 1 flags
)

var magic = [4]byte{0x50, 0x4C, 0x47, 0x4E}

type conn struct {
	c net.Conn
}

// lockedConn wraps conn with a mutex for serialised send/recv round-trips.
type lockedConn struct {
	mu sync.Mutex
	conn
}

func newConn(c net.Conn) Conn {
	return &conn{c: c}
}

func newLockedConn(c net.Conn) *lockedConn {
	return &lockedConn{conn: conn{c: c}}
}

func (c *conn) Send(msgType MsgType, payload []byte) error {
	if len(payload) > maxFrameSize {
		return fmt.Errorf("payload %d bytes exceeds max frame size", len(payload))
	}
	var hdr [headerSize]byte
	copy(hdr[:4], magic[:])
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(payload))) //nolint:gosec // len is bounded by maxFrameSize check above
	hdr[8] = byte(msgType)
	if len(payload) == 0 {
		n, err := c.c.Write(hdr[:])
		if err != nil {
			return err
		}
		if n != headerSize {
			return io.ErrShortWrite
		}
		return nil
	}

	parts := [2][]byte{hdr[:], payload}
	bufs := net.Buffers(parts[:])
	n, err := bufs.WriteTo(c.c)
	if err != nil {
		return err
	}
	if want := int64(headerSize + len(payload)); n != want {
		return io.ErrShortWrite
	}
	return nil
}

func (c *conn) Recv() (MsgType, []byte, error) {
	msgType, payload, _, err := c.recvInto(nil)
	return msgType, payload, err
}

func (c *conn) recvInto(buf []byte) (MsgType, []byte, []byte, error) {
	var hdr [headerSize]byte
	if _, err := io.ReadFull(c.c, hdr[:]); err != nil {
		return 0, nil, buf, err
	}
	if hdr[0] != magic[0] || hdr[1] != magic[1] || hdr[2] != magic[2] || hdr[3] != magic[3] {
		return 0, nil, buf, fmt.Errorf("invalid magic bytes")
	}
	length := binary.LittleEndian.Uint32(hdr[4:8])
	if length > maxFrameSize {
		return 0, nil, buf, fmt.Errorf("frame length %d exceeds max frame size", length)
	}
	msgType := MsgType(hdr[8])
	if cap(buf) < int(length) {
		buf = make([]byte, length)
	}
	payload := buf[:length]
	if _, err := io.ReadFull(c.c, payload); err != nil {
		return 0, nil, buf, err
	}
	return msgType, payload, buf, nil
}

func (c *conn) Close() error {
	return c.c.Close()
}
