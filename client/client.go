package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/kevin-chtw/tw_client/message"
	"github.com/kevin-chtw/tw_client/packet"
	"github.com/sirupsen/logrus"
	"github.com/topfreegames/pitaya/v3/pkg/conn/codec"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"

	"github.com/topfreegames/pitaya/v3/pkg/util/compression"
)

// HandshakeSys struct
type HandshakeSys struct {
	Dict       map[string]uint16 `json:"dict"`
	Heartbeat  int               `json:"heartbeat"`
	Serializer string            `json:"serializer"`
}

// HandshakeData struct
type HandshakeData struct {
	Code int          `json:"code"`
	Sys  HandshakeSys `json:"sys"`
}

type sendTask struct {
	msg *message.Message
	// 对于 Request 用 respCh 回传结果；Notify 无回调
	respCh chan *message.Message
}

// 客户端结构体
type Client struct {
	conn      net.Conn
	stopCh    chan struct{}
	sendCh    chan sendTask
	Connected bool
	pendMu    sync.Mutex
	pending   map[uint]chan *message.Message
	idSeq     atomic.Uint32
	onMsg     func(*message.Message)
}

// 创建客户端
func NewClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn:    conn,
		stopCh:  make(chan struct{}),
		sendCh:  make(chan sendTask, 128),
		pending: make(map[uint]chan *message.Message),
	}

	if err := c.handleHandshake(); err != nil {
		conn.Close()
		return nil, err
	}

	go c.readLoop()
	go c.writeLoop()
	return c, nil
}

// OnMessage 设置收到推送/广播时的回调
func (c *Client) OnMessage(fn func(*message.Message)) { c.onMsg = fn }

// Notify 发一条不需要回包的消息（支持 protobuf）
func (c *Client) Notify(route string, msg proto.Message) error {
	b, _ := proto.Marshal(msg)
	return c.NotifyBytes(route, b)
}

func (c *Client) NotifyBytes(route string, data []byte) error {
	m := &message.Message{Type: message.Notify, Route: route, Data: data}
	select {
	case c.sendCh <- sendTask{msg: m}:
		return nil
	case <-c.stopCh:
		return errors.New("client stopped")
	}
}

// Request 发一条需要回包的消息，返回 *Message future
func (c *Client) Request(route string, msg proto.Message) (<-chan *message.Message, error) {
	b, _ := proto.Marshal(msg)
	return c.RequestBytes(route, b)
}

func (c *Client) RequestBytes(route string, data []byte) (<-chan *message.Message, error) {
	id := c.idSeq.Add(1)
	m := &message.Message{Type: message.Request, ID: uint(id), Route: route, Data: data}
	ch := make(chan *message.Message, 1)

	c.pendMu.Lock()
	c.pending[uint(id)] = ch
	c.pendMu.Unlock()

	select {
	case c.sendCh <- sendTask{msg: m, respCh: ch}:
		return ch, nil
	case <-c.stopCh:
		return nil, errors.New("client stopped")
	}
}

// --------------------------------------------------
// 读写循环
// --------------------------------------------------
func (c *Client) readLoop() {
	defer c.Close()
	buf := bytes.NewBuffer(nil)
	for {
		pkt, err := c.readPackets(buf)
		if err != nil {
			return
		}
		for _, pkt := range pkt {
			switch pkt.Type {
			case packet.Heartbeat:
				_ = c.NotifyBytes("sys.heartbeat", []byte("{}"))
			case packet.Kick:
				return
			case packet.Data:
				msg, err := message.Decode(pkt.Data)
				if err != nil {
					return
				}
				if msg.Type == message.Response {
					c.pendMu.Lock()
					ch := c.pending[msg.ID]
					delete(c.pending, msg.ID)
					c.pendMu.Unlock()
					if ch != nil {
						ch <- msg
					}
				} else if c.onMsg != nil {
					c.onMsg(msg)
				}
			}
		}
	}
}

func (c *Client) writeLoop() {
	for {
		select {
		case <-c.stopCh:
			return
		case task := <-c.sendCh:
			if task.msg.Route == "sys.heartbeat" {
				buf, err := packet.Encode(packet.Data, []byte(""))
				if err != nil {
					logrus.Error("Failed to encode packet:", err.Error())
					return
				}
				if _, err = c.conn.Write(buf); err != nil {
					logrus.Error("Failed to send message:", err.Error())
					return
				}
			}
			data, err := message.Encode(task.msg)
			if err != nil {
				return
			}

			buf, err := packet.Encode(packet.Data, data)
			if err != nil {
				logrus.Error("Failed to encode packet:", err.Error())
				return
			}

			if _, err = c.conn.Write(buf); err != nil {
				logrus.Error("Failed to send message:", err.Error())
				return
			}
		}
	}
}

// 关闭连接
func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) handleHandshake() error {
	if err := c.sendHandshakeRequest(); err != nil {
		return err
	}

	if err := c.handleHandshakeResponse(); err != nil {
		return err
	}
	return nil
}

func (c *Client) sendHandshakeRequest() error {
	enc, err := json.Marshal(sessionHandshake)
	if err != nil {
		return err
	}

	p, err := packet.Encode(packet.Handshake, enc)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(p)
	return err
}

func (c *Client) handleHandshakeResponse() error {
	buf := bytes.NewBuffer(nil)
	packets, err := c.readPackets(buf)
	if err != nil {
		return err
	}

	handshakePacket := packets[0]
	if handshakePacket.Type != packet.Handshake {
		return fmt.Errorf("got first packet from server that is not a handshake, aborting")
	}

	handshake := &HandshakeData{}
	if compression.IsCompressed(handshakePacket.Data) {
		handshakePacket.Data, err = compression.InflateData(handshakePacket.Data)
		if err != nil {
			return err
		}
	}

	err = json.Unmarshal(handshakePacket.Data, handshake)
	if err != nil {
		return err
	}

	log.Printf("got handshake from sv, data: %v", handshake)

	p, err := packet.Encode(packet.HandshakeAck, []byte{})
	if err != nil {
		return err
	}
	_, err = c.conn.Write(p)
	if err != nil {
		return err
	}

	c.Connected = true

	return nil
}

func (c *Client) readPackets(buf *bytes.Buffer) ([]*packet.Packet, error) {
	// listen for sv messages
	data := make([]byte, 1024)
	n := len(data)
	var err error

	for n == len(data) {
		n, err = c.conn.Read(data)
		if err != nil {
			return nil, err
		}
		buf.Write(data[:n])
	}
	packets, err := packet.Decode(buf.Bytes())
	if err != nil {
		logger.Log.Errorf("error decoding packet from server: %s", err.Error())
	}
	totalProcessed := 0
	for _, p := range packets {
		totalProcessed += codec.HeadLength + p.Length
	}
	buf.Next(totalProcessed)

	return packets, nil
}
