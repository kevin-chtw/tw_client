package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kevin-chtw/tw_client/client"
	"github.com/kevin-chtw/tw_client/message"
)

func main() {
	// 创建客户端
	client, err := client.NewClient("localhost:3250")
	if err != nil {
		log.Fatal("Failed to connect to server:", err)
	}
	defer client.Close()

	// 创建并发送消息
	msg := &message.Message{
		Type:  message.Request,
		ID:    123,
		Route: "lobby.lobby.playermsg",
		Data:  []byte("{\"login_req\": {\"username\": \"testuser\", \"password\": \"testpass\"}}"),
	}

	err = client.Send(msg)
	if err != nil {
		log.Println("Failed to send message:", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	// 创建一个上下文，用于超时处理（可选）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Hour)
	defer cancel()

	for {
		// 等待信号
		select {
		case <-sigCh:
			fmt.Println("收到终止信号，开始清理资源...")
			os.Exit(0)
		case <-ctx.Done():
			fmt.Println("超时，强制退出...")
			os.Exit(1)
		default:
			client.Receive()
		}
	}
}
