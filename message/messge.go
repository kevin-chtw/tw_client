package message

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/topfreegames/pitaya/v3/pkg/util/compression"
)

// 定义消息类型和掩码
type Type byte

const (
	Request  Type = 0x00
	Notify   Type = 0x01
	Response Type = 0x02
	Push     Type = 0x03

	errorMask            = 0x20
	gzipMask             = 0x10
	msgRouteCompressMask = 0x01
	msgTypeMask          = 0x07
	msgRouteLengthMask   = 0xFF
	msgHeadLength        = 0x02
)

var (
	routesCodesMutex = sync.RWMutex{}
	routes           = make(map[string]uint16) // route map to code
	codes            = make(map[uint16]string) // code map to route
)

// 错误定义
var (
	ErrWrongMessageType  = errors.New("wrong message type")
	ErrInvalidMessage    = errors.New("invalid message")
	ErrRouteInfoNotFound = errors.New("route info not found in dictionary")
)

// 消息结构体
type Message struct {
	Type       Type
	ID         uint
	Route      string
	Data       []byte
	compressed bool
	Err        bool
}

func routable(t Type) bool {
	return t == Request || t == Notify || t == Push
}

func invalidType(t Type) bool {
	return t < Request || t > Push

}

// 编码消息为字节流
func Encode(msg *Message) ([]byte, error) {
	if invalidType(msg.Type) {
		return nil, ErrWrongMessageType
	}
	buf := make([]byte, 0)
	flag := byte(msg.Type) << 1

	routesCodesMutex.RLock()
	code, compressed := routes[msg.Route]
	routesCodesMutex.RUnlock()
	if compressed {
		flag |= msgRouteCompressMask
	}

	if msg.Err {
		flag |= errorMask
	}

	buf = append(buf, flag)

	if msg.Type == Request || msg.Type == Response {
		n := msg.ID
		// variant length encode
		for {
			b := byte(n % 128)
			n >>= 7
			if n != 0 {
				buf = append(buf, b+128)
			} else {
				buf = append(buf, b)
				break
			}
		}
	}

	if routable(msg.Type) {
		if compressed {
			buf = append(buf, byte((code>>8)&0xFF))
			buf = append(buf, byte(code&0xFF))
		} else {
			buf = append(buf, byte(len(msg.Route)))
			buf = append(buf, []byte(msg.Route)...)
		}
	}

	buf = append(buf, msg.Data...)
	return buf, nil
}

// 解码字节流为消息
func Decode(data []byte) (*Message, error) {
	if len(data) < msgHeadLength {
		return nil, ErrInvalidMessage
	}
	m := &Message{}
	flag := data[0]
	offset := 1
	m.Type = Type((flag >> 1) & msgTypeMask)

	if invalidType(m.Type) {
		return nil, ErrWrongMessageType
	}

	if m.Type == Request || m.Type == Response {
		id := uint(0)
		// little end byte order
		// WARNING: must can be stored in 64 bits integer
		// variant length encode
		for i := offset; i < len(data); i++ {
			b := data[i]
			id += uint(b&0x7F) << uint(7*(i-offset))
			if b < 128 {
				offset = i + 1
				break
			}
		}
		m.ID = id
	}

	m.Err = flag&errorMask == errorMask

	size := len(data)
	if routable(m.Type) {
		if flag&msgRouteCompressMask == 1 {
			if offset > size || (offset+2) > size {
				return nil, ErrInvalidMessage
			}

			m.compressed = true
			code := binary.BigEndian.Uint16(data[offset:(offset + 2)])
			routesCodesMutex.RLock()
			route, ok := codes[code]
			routesCodesMutex.RUnlock()
			if !ok {
				return nil, ErrRouteInfoNotFound
			}
			m.Route = route
			offset += 2
		} else {
			m.compressed = false
			rl := data[offset]
			offset++

			if offset > size || (offset+int(rl)) > size {
				return nil, ErrInvalidMessage
			}
			m.Route = string(data[offset:(offset + int(rl))])
			offset += int(rl)
		}
	}

	if offset > size {
		return nil, ErrInvalidMessage
	}

	m.Data = data[offset:]
	var err error
	if flag&gzipMask == gzipMask {
		m.Data, err = compression.InflateData(m.Data)
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}
