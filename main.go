package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kevin-chtw/tw_client/client"
	"github.com/kevin-chtw/tw_proto/cproto"
)

func main() {
	// 创建客户端
	cli, err := client.NewClient("localhost:3250")
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	defer cli.Close()

	// 处理控制台输入
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n请选择操作:")
		fmt.Println("1. 登录")
		fmt.Println("2. 注册")
		fmt.Println("3. 退出")
		fmt.Print("请输入选项: ")

		input, _ := reader.ReadString('\n')
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			login(cli, reader)
		case "2":
			register(cli, reader)
		case "3":
			fmt.Println("退出程序...")
			os.Exit(0)
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
	req := &cproto.LoginReq{
		Account:  account,
		Password: password,
	}

	// 发送登录请求
	res, err := cli.Login(req)
	if err != nil {
		fmt.Printf("登录失败: %v\n", err)
		return
	}

	fmt.Printf("登录成功! 用户ID: %s, 服务器ID: %s\n", res.Serverid, res.Userid)
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
	res, err := cli.Register(req)
	if err != nil {
		fmt.Printf("注册失败: %v\n", err)
		return
	}

	fmt.Printf("注册成功! 用户ID: %s\n", res.RegisterAck.Userid)
}
