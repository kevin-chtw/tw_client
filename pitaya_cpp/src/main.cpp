#include <iostream>
#include <chrono>
#include <cstdint>
#include "client.h"
#include "message.h"
#include "lobby.pb.h"
#include "google/protobuf/util/json_util.h" // 引入JSON工具库

void OnMessage(const pitaya::Message &msg)
{
    std::cout << "Received message: type=" << static_cast<int>(msg.type) << ", router=" << msg.route
              << ",data" << std::string(msg.data.begin(), msg.data.end()) << std::endl;
}

int main()
{
    pitaya::Client client;
    if (!client.Start("127.0.0.1", 3250))
    {
        std::cerr << "Failed to connect" << std::endl;
        return 1;
    }

    client.OnMessage(OnMessage);

    cproto::LobbyReq req;
    auto msg = req.mutable_login_req();
    msg->set_account("test");
    msg->set_password("1111111");

    std::string jsonStr = req.SerializeAsString();
    auto fut = client.Request("lobby.player.message", req);
    auto resp = fut.get();
    cproto::LobbyAck ack;
    if (ack.ParseFromString(std::string(resp.data.begin(), resp.data.end())))
    {
        std::cout << "Response: " << std::string(ack.DebugString()) << std::endl;
    }

    while (!client.IsStop())
    {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    return 0;
}