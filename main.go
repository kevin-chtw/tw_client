package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/kevin-chtw/tw_client/client"
	"github.com/kevin-chtw/tw_client/message"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var g_client *client.Client

func OnMessage(msg *message.Message) {
	logrus.Infof("收到消息:%v\n", msg.Route)
	if msg.Route == "fdtable" {
		resMsg := &cproto.MatchAck{}
		if err := proto.Unmarshal(msg.Data, resMsg); err != nil {
			fmt.Printf("解析响应失败: %v\n", err)
			return
		}
		logrus.Infof("收到消息:%v", resMsg)
		ack, err := resMsg.Ack.UnmarshalNew()
		if err != nil {
			fmt.Printf("解析响应失败: %v\n", err)
			return
		}
		logrus.Infof("收到消息:%v", resMsg.Ack.TypeUrl)
		if resMsg.Ack.TypeUrl == "type.googleapis.com/cproto.CreateRoomAck" {
			ackMsg := ack.(*cproto.CreateRoomAck)
			EnterGame(ackMsg.Tableid)
		}
		if resMsg.Ack.TypeUrl == "type.googleapis.com/cproto.JoinRoomAck" {
			ackMsg := ack.(*cproto.JoinRoomAck)
			EnterGame(ackMsg.Tableid)
		}
	}
	if msg.Route == "lobby" {
		resMsg := &cproto.LobbyAck{}
		if err := proto.Unmarshal(msg.Data, resMsg); err != nil {
			fmt.Printf("解析响应失败: %v\n", err)
			return
		}
		logrus.Infof("收到消息:%v", resMsg)
	}
	if msg.Route == "mjhaeb" {
		resMsg := &cproto.GameAck{}
		if err := proto.Unmarshal(msg.Data, resMsg); err != nil {
			fmt.Printf("解析响应失败: %v\n", err)
			return
		}
		logrus.Infof("收到消息:%v", resMsg)
	}
}

func main() {
	// 创建客户端
	cli, err := client.NewClient("localhost:3250")
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	defer cli.Close()
	g_client = cli

	cli.OnMessage(OnMessage)
	// 处理控制台输入
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("请选择操作:q.退出 1.登录 2.注册 3.创建房间 4.加入房间")
		fmt.Println("请输入选项: ")

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "q":
			fmt.Println("退出程序...")
			os.Exit(0)
		case "1":
			login(cli, reader)
		case "2":
			register(cli, reader)
		case "3":
			CreateRoom(cli, reader)
		case "4":
			JoinRoom(cli, reader)
		default:
			fmt.Println("无效选项，请重新输入")
		}
	}
}

func login(cli *client.Client, reader *bufio.Reader) {
	fmt.Print("请输入账号: ")
	account, _ := reader.ReadString('\n')
	account = strings.TrimSpace(account)

	fmt.Print("请输入密码: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	// 构建登录请求
	req := &cproto.LobbyReq{
		LoginReq: &cproto.LoginReq{
			Account:  account,
			Password: password,
		},
	}

	// 发送登录请求
	res, err := cli.Request("lobby.player.message", req)
	if err != nil {
		fmt.Printf("登录失败: %v\n", err)
		return
	}

	fmt.Printf("登录成功! %v\n", res)
}

func register(cli *client.Client, reader *bufio.Reader) {
	fmt.Print("请输入账号: ")
	account, _ := reader.ReadString('\n')
	account = strings.TrimSpace(account)

	fmt.Print("请输入密码: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	// 构建注册请求
	req := &cproto.LobbyReq{
		RegisterReq: &cproto.RegisterReq{
			Account:  account,
			Password: password,
		},
	}

	// 发送注册请求
	res, err := cli.Request("lobby.player.message", req)
	if err != nil {
		fmt.Printf("注册失败: %v\n", err)
		return
	}

	fmt.Printf("注册成功! 用户ID: %v\n", res)
}

func CreateRoom(cli *client.Client, reader *bufio.Reader) {
	createReq := &cproto.CreateRoomReq{
		GameCount: 1,
		Desn:      "test",
	}
	anyReq, err := anypb.New(createReq)
	if err != nil {
		return
	}
	req := &cproto.MatchReq{
		Matchid: 1001001,
		Req:     anyReq,
	}

	// 发送创建房间请求
	resCh, err := cli.Request("fdtable.player.message", req)
	if err != nil {
		fmt.Printf("创建房间失败: %v\n", err)
		return
	}

	// 等待响应
	res := <-resCh
	if res == nil {
		fmt.Println("创建房间失败: 未收到响应")
		return
	}

	resMsg := &cproto.MatchAck{}
	if err := proto.Unmarshal(res.Data, resMsg); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	ack, err := resMsg.Ack.UnmarshalNew()
	if err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}
	logrus.Infof("收到消息:%v", resMsg.Ack.TypeUrl)
	ackMsg := ack.(*cproto.CreateRoomAck)
	EnterGame(ackMsg.Tableid)

}

func EnterGame(tableId int32) {
	fmt.Printf("进入房间: %v\n", tableId)

	createReq := &cproto.EnterGameReq{}
	anyReq, err := anypb.New(createReq)
	if err != nil {
		return
	}
	req := &cproto.GameReq{
		Gameid:  1001,
		Matchid: 1001001,
		Tableid: int32(tableId),
		Req:     anyReq,
	}

	// 发送创建房间请求
	_, err = g_client.Request("mjhaeb.player.message", req)
	if err != nil {
		fmt.Printf("进入房间失败: %v\n", err)
		return
	}
	fmt.Printf("进入房间成功: %v\n", tableId)
}

func JoinRoom(cli *client.Client, reader *bufio.Reader) {
	fmt.Print("请输入房间号: ")
	tableId, _ := reader.ReadString('\n')
	tableId = strings.TrimSpace(tableId)
	tableIdInt, err := strconv.Atoi(tableId)
	if err != nil {
		fmt.Printf("无效的桌子ID: %v\n", err)
		return
	}
	JoinRoom := &cproto.JoinRoomReq{
		Tableid: int32(tableIdInt),
	}
	anyReq, err := anypb.New(JoinRoom)
	if err != nil {
		return
	}
	req := &cproto.MatchReq{
		Matchid: 1001001,
		Req:     anyReq,
	}

	// 发送创建房间请求
	resCh, err := cli.Request("fdtable.player.message", req)
	if err != nil {
		fmt.Printf("创建房间失败: %v\n", err)
		return
	}

	// 等待响应
	res := <-resCh
	if res == nil {
		fmt.Println("创建房间失败: 未收到响应")
		return
	}

	resMsg := &cproto.MatchAck{}
	if err := proto.Unmarshal(res.Data, resMsg); err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}

	ack, err := resMsg.Ack.UnmarshalNew()
	if err != nil {
		fmt.Printf("解析响应失败: %v\n", err)
		return
	}
	ackMsg := ack.(*cproto.JoinRoomAck)
	EnterGame(ackMsg.Tableid)
}
