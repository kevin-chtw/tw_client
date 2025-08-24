#include <iostream>
#include <chrono>
#include <cstdint>
#include "client.h"
#include "message.h"

void OnMessage(const pitaya::Message& msg) {
    std::cout << "Received message: type=" << static_cast<int>(msg.type) << ", router=" << msg.route 
    <<",data" << std::string(msg.data.begin(), msg.data.end())<< std::endl;
}

int main() {
    pitaya::Client client;
    if (!client.Start("127.0.0.1", 3250)) {
        std::cerr << "Failed to connect" << std::endl;
        return 1;
    } 
    
    client.OnMessage(OnMessage);
    
    // 将JSON字符串转换为vector<uint8_t>
    std::string jsonStr = "{\"login_req\":{\"account\":\"test\",\"password\":\"1111111\"}}";
    
    auto fut = client.Request("lobby.player.message",std::vector<uint8_t>(jsonStr.begin(), jsonStr.end()));
    auto resp = fut.get();
    std::cout << "Response: " << std::string(resp.data.begin(), resp.data.end()) << std::endl;

    while(!client.IsStop()){
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    return 0;
}