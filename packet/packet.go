package packet

import (
	"bytes"
	"errors"
)

type Type byte

const (
	_ Type = iota
	// Handshake represents a handshake: request(client) <====> handshake response(server)
	Handshake = 0x01

	// HandshakeAck represents a handshake ack from client to server
	HandshakeAck = 0x02

	// Heartbeat represents a heartbeat
	Heartbeat = 0x03

	// Data represents a common data packet
	Data = 0x04

	// Kick represents a kick off packet
	Kick = 0x05 // disconnect message from server
)

// Codec constants.
const (
	HeadLength    = 4
	MaxPacketSize = 1 << 24 //16MB
)

// ErrWrongPomeloPacketType represents a wrong packet type.
var ErrWrongPomeloPacketType = errors.New("wrong packet type")

// ErrInvalidPomeloHeader represents an invalid header
var ErrInvalidPomeloHeader = errors.New("invalid header")
var ErrPacketSizeExcced = errors.New("codec: packet size exceed")

type Packet struct {
	Type   Type
	Length int
	Data   []byte
}

func Encode(typ Type, data []byte) ([]byte, error) {
	if typ < Handshake || typ > Kick {
		return nil, ErrWrongPomeloPacketType
	}

	if len(data) > MaxPacketSize {
		return nil, ErrPacketSizeExcced
	}

	p := &Packet{Type: typ, Length: len(data)}
	buf := make([]byte, p.Length+HeadLength)
	buf[0] = byte(p.Type)

	copy(buf[1:HeadLength], IntToBytes(p.Length))
	copy(buf[HeadLength:], data)

	return buf, nil

}

// Decode decode the network bytes slice to packet.Packet(s)
func Decode(data []byte) ([]*Packet, error) {
	buf := bytes.NewBuffer(nil)
	buf.Write(data)

	var (
		packets []*Packet
		err     error
	)
	// check length
	if buf.Len() < HeadLength {
		return nil, nil
	}

	// first time
	size, typ, err := forward(buf)
	if err != nil {
		return nil, err
	}

	for size <= buf.Len() {
		p := &Packet{Type: typ, Length: size, Data: buf.Next(size)}
		packets = append(packets, p)

		// if no more packets, break
		if buf.Len() < HeadLength {
			break
		}

		size, typ, err = forward(buf)
		if err != nil {
			return nil, err
		}
	}

	return packets, nil
}

// IntToBytes encode packet data length to bytes(Big end)
func IntToBytes(n int) []byte {
	buf := make([]byte, 3)
	buf[0] = byte((n >> 16) & 0xFF)
	buf[1] = byte((n >> 8) & 0xFF)
	buf[2] = byte(n & 0xFF)
	return buf
}

func forward(buf *bytes.Buffer) (int, Type, error) {
	header := buf.Next(HeadLength)
	return ParseHeader(header)
}

// ParseHeader parses a packet header and returns its dataLen and packetType or an error
func ParseHeader(header []byte) (int, Type, error) {
	if len(header) != HeadLength {
		return 0, 0x00, ErrInvalidPomeloHeader
	}
	typ := header[0]
	if typ < Handshake || typ > Kick {
		return 0, 0x00, ErrWrongPomeloPacketType
	}

	size := BytesToInt(header[1:])

	if size > MaxPacketSize {
		return 0, 0x00, ErrPacketSizeExcced
	}

	return size, Type(typ), nil
}

// BytesToInt decode packet data length byte to int(Big end)
func BytesToInt(b []byte) int {
	result := 0
	for _, v := range b {
		result = result<<8 + int(v)
	}
	return result
}
