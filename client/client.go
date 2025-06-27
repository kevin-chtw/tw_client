package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/kevin-chtw/tw_client/message"
	"github.com/kevin-chtw/tw_client/packet"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/topfreegames/pitaya/v3/pkg/conn/codec"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
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

type pendingRequest struct {
	msg    *message.Message
	sentAt time.Time
}

// 客户端结构体
type Client struct {
	conn      net.Conn
	Connected bool
	UserID    string
	ServerID  string
}

// 创建客户端
func NewClient(addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{conn: conn}
	if err := c.handleHandshake(); err != nil {
		return nil, err
	}
	return c, nil
}

// Login 发送登录请求并处理响应
func (c *Client) Login(req *cproto.LoginReq) (*cproto.LoginAck, error) {
	// 序列化请求为JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化登录请求失败: %v", err)
	}

	msg := &message.Message{
		Type:  message.Request,
		ID:    uint(time.Now().UnixNano() & 0xFFFFFFFF),
		Route: "account.account.verify",
		Data:  reqData,
	}
	log.Printf("Sending request message: Type=%d, Route=%s", msg.Type, msg.Route)

	// 发送请求
	if err := c.Send(msg); err != nil {
		return nil, fmt.Errorf("发送登录请求失败: %v", err)
	}

	// 接收响应
	resp, err := c.Receive()
	if err != nil {
		return nil, fmt.Errorf("接收登录响应失败: %v", err)
	}

	// 反序列化响应
	ack := &cproto.LoginAck{}
	if err := json.Unmarshal(resp.Data, ack); err != nil {
		return nil, fmt.Errorf("解析登录响应失败: %v", err)
	}

	if ack.Userid == "" {
		return nil, fmt.Errorf("登录失败")
	}

	c.UserID = ack.Userid
	c.ServerID = ack.Serverid
	return ack, nil
}

// Register 发送注册请求并处理响应
func (c *Client) Register(req *cproto.LobbyReq) (*cproto.LobbyAck, error) {
	// 序列化请求为JSON
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化注册请求失败: %v", err)
	}

	msg := &message.Message{
		Type:  message.Request,
		ID:    uint(time.Now().UnixNano() & 0xFFFFFFFF),
		Route: "lobby.lobby.playermsg",
		Data:  reqData,
	}
	log.Printf("Sending request message: Type=%d, Route=%s", msg.Type, msg.Route)

	// 发送请求
	if err := c.Send(msg); err != nil {
		return nil, fmt.Errorf("发送注册请求失败: %v", err)
	}

	// 接收响应
	resp, err := c.Receive()
	if err != nil {
		return nil, fmt.Errorf("接收注册响应失败: %v", err)
	}

	// 反序列化响应
	ack := &cproto.LobbyAck{}
	if err := json.Unmarshal(resp.Data, ack); err != nil {
		return nil, fmt.Errorf("解析注册响应失败: %v", err)
	}

	if ack.RegisterAck == nil || ack.RegisterAck.Userid == "" {
		return nil, fmt.Errorf("注册失败")
	}

	c.UserID = ack.RegisterAck.Userid
	return ack, nil
}
func (c *Client) Send(msg *message.Message) error {
	data, err := message.Encode(msg)
	if err != nil {
		return err
	}

	buf, err := packet.Encode(packet.Data, data)
	if err != nil {
		log.Println("Failed to encode packet:", err)
		return err
	}

	if _, err = c.conn.Write(buf); err != nil {
		log.Println("Failed to send message:", err)
	}
	return err
}

// 接收消息
func (c *Client) Receive() (*message.Message, error) {
	buf := make([]byte, 1024)
	n, err := c.conn.Read(buf)
	if err != nil {
		if err == io.EOF {
			return nil, message.ErrInvalidMessage
		}
		return nil, err
	}

	pkts, err := packet.Decode(buf[:n])
	if err != nil {
		return nil, fmt.Errorf("packet decode failed: %v", err)
	}

	for _, pkt := range pkts {
		if pkt.Type != packet.Data {
			continue
		}

		msg, err := message.Decode(pkt.Data)
		if err != nil {
			return nil, fmt.Errorf("message decode failed: %v", err)
		}

		log.Printf("Received message: Type=%d, ID=%d, Route=%s\n", msg.Type, msg.ID, msg.Route)
		return msg, nil
	}

	return nil, message.ErrInvalidMessage
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
