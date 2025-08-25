#pragma once
#include <string>
#include <memory>
#include <vector>
#include <map>
#include <chrono>
#include <nlohmann/json.hpp>
#include "message.h"
#include "packet.h"
#include <future>
#include <thread>
#include <queue>
#include <mutex>
#include <condition_variable>
#include <atomic>
#include <google/protobuf/message.h>

namespace pitaya
{

    // ========== 与 Go 侧一致的 JSON 结构 ==========
    struct HandshakeClientData
    {
        std::string platform;
        std::string libVersion;
        std::string buildNumber;
        std::string version;
        NLOHMANN_DEFINE_TYPE_INTRUSIVE(HandshakeClientData,
                                       platform, libVersion, buildNumber, version)
    };

    struct SessionHandshakeData
    {
        HandshakeClientData sys;
        std::map<std::string, nlohmann::json> user;
        NLOHMANN_DEFINE_TYPE_INTRUSIVE(SessionHandshakeData, sys, user)
    };

    struct HandshakeSys
    {
        std::map<std::string, uint16_t> dict;
        int heartbeat = 0;
        std::string serializer;
        NLOHMANN_DEFINE_TYPE_INTRUSIVE(HandshakeSys, dict, heartbeat, serializer)
    };

    struct HandshakeData
    {
        int code = 0;
        HandshakeSys sys;
        NLOHMANN_DEFINE_TYPE_INTRUSIVE(HandshakeData, code, sys)
    };

    // ========== 客户端 ==========
    class Client
    {
    public:
        Client() = default;
        ~Client();

        // 连接服务器
        bool Start(const std::string &host, uint16_t port);
        void Stop();

        // 业务线程直接调用，无阻塞
        bool Notify(const std::string &route, const google::protobuf::Message &msg);
        bool Notify(const std::string &route, const std::vector<uint8_t> &data);

        std::future<Message> Request(const std::string &route, const google::protobuf::Message &msg);
        std::future<Message> Request(const std::string &route, const std::vector<uint8_t> &data);

        // 回调注册线程安全）
        void OnMessage(std::function<void(const Message &)> cb);

        // 关闭连接
        void Close();

        const std::string &UserID() const { return user_id_; }
        const std::string &ServerID() const { return server_id_; }
        bool IsStop() const { return stop_; }

    private:
        // 生成唯一 msg id
        uint32_t NextMsgID();
        bool sendHandshakeRequest();
        bool handleHandshakeResponse();

        /* ---------- 接收相关 ---------- */
        void RecvThreadFunc();
        void SendThreadFunc();
        bool readPackets(std::vector<uint8_t> &buffer, std::vector<Packet> &out);

        ssize_t recvSome(std::vector<uint8_t> &buffer);

    private:
        int sock_ = -1;
        std::atomic<bool> stop_{false};
        std::string user_id_;
        std::string server_id_;

        std::thread recv_th_;
        std::thread send_th_;

        /* ---------- 发送队列 ---------- */
        struct SendItem
        {
            Message msg;
            std::promise<bool> prom;        // Notify 用
            std::promise<Message> prom_req; // Request 用
        };
        std::queue<SendItem> send_q_;
        std::mutex send_mtx_;
        std::condition_variable send_cv_;

        // 挂起的 Request
        std::mutex pending_mtx_;
        std::unordered_map<uint32_t, std::promise<Message>> pending_;
        uint32_t id_seq_ = 0;

        std::function<void(const Message &)> on_message_;
    };

} // namespace pomelo_client