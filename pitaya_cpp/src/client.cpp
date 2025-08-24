#include "client.h"
#include "packet.h"
#include <arpa/inet.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/select.h>
#include <cstring>
#include <iostream>
#include <stdexcept>
#include <thread>

namespace pitaya {

namespace {
    // 默认握手 payload（对应 Go 的 sessionHandshake）
    nlohmann::json defaultHandshake() {
        SessionHandshakeData h;
        h.sys.platform    = "repl";
        h.sys.libVersion  = "0.3.5-release";
        h.sys.buildNumber = "20";
        h.sys.version     = "1.0.0";
        h.user["client"]  = "repl";
        return h;
    }
}

Client::~Client() { Close(); }

bool Client::Start(const std::string& host, uint16_t port) {
    sock_ = socket(AF_INET, SOCK_STREAM, 0);
    if (sock_ < 0) {
        std::cerr << "socket failed\n";
        return false;
    }

    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_port   = htons(port);
    if (inet_pton(AF_INET, host.c_str(), &addr.sin_addr) <= 0) {
        std::cerr << "invalid address\n";
        return false;
    }

    if (::connect(sock_, reinterpret_cast<sockaddr*>(&addr), sizeof(addr)) < 0) {
        std::cerr << "connect failed\n";
        return false;
    }

    if (!sendHandshakeRequest()) return false;
    if (!handleHandshakeResponse()) return false;

    stop_  = false;
    recv_th_ = std::thread(&Client::RecvThreadFunc, this);
    send_th_ = std::thread(&Client::SendThreadFunc, this);
    return true;
}

void Client::Stop() {
    stop_ = true;
    ::shutdown(sock_, SHUT_RDWR);
    send_cv_.notify_all();
    if (recv_th_.joinable()) recv_th_.join();
    if (send_th_.joinable()) send_th_.join();
    Close();
}

void Client::Close() {
    if (sock_ >= 0) {
        ::close(sock_);
        sock_ = -1;
    }
}

// ---------------- 发送接口 ----------------
bool Client::Notify(const std::string& route, const std::vector<uint8_t>& data) {
    Message m;
    m.type  = MessageType::Notify;
    m.route = route;
    m.data  = data;

    SendItem item;
    item.msg = std::move(m);
    std::future<bool> fut = item.prom.get_future();

    {
        std::lock_guard<std::mutex> lk(send_mtx_);
        send_q_.push(std::move(item));
    }
    send_cv_.notify_one();
    return fut.get();   // 等待发送线程写完
}

std::future<Message> Client::Request(const std::string& route, const std::vector<uint8_t>& data) {
    uint32_t id = NextMsgID();
    Message m;
    m.type  = MessageType::Request;
    m.id    = id;
    m.route = route;
    m.data  = data;

    SendItem item;
    item.msg = std::move(m);
    std::future<Message> fut = item.prom_req.get_future();

    {
        std::lock_guard<std::mutex> lk(send_mtx_);
        send_q_.push(std::move(item));
    }
    send_cv_.notify_one();
    return fut;
}

void Client::OnMessage(std::function<void(const Message&)> cb) {
    on_message_ = std::move(cb);
}

// ---------------- 发送线程 ----------------
void Client::SendThreadFunc() {
    while (!stop_) {
        SendItem task;
        {
            std::unique_lock<std::mutex> lk(send_mtx_);
            send_cv_.wait(lk, [this]{ return !send_q_.empty() || stop_; });
            if (stop_) {
                std::cout << "SendThread stopping\n";
                break;
            }
            task = std::move(send_q_.front());
            send_q_.pop();
        }

    auto encodedMsg = MessageCodec::Encode(task.msg);
    auto pkt = Codec::Encode(PacketType::Data, encodedMsg);
    bool ok = ::write(sock_, pkt.data(), pkt.size()) == (ssize_t)pkt.size();

        if (task.msg.type == MessageType::Request) {
            // 记录 promise
            std::lock_guard<std::mutex> lk(pending_mtx_);
            pending_.emplace(task.msg.id, std::move(task.prom_req));
        } else {
            task.prom.set_value(ok);
        }
    }
}

// ---------------- 接收线程 ----------------
void Client::RecvThreadFunc() {
    std::vector<uint8_t> buf;
    std::vector<Packet> pkts;
    while (!stop_) {
        if (!readPackets(buf, pkts)) { 
            std::cerr << "RecvThread readPackets failed\n";
            break; 
        }

        for (auto& p : pkts) {
            if(p.type == PacketType::Heartbeat) {
                std::string strAck = "{}";
                Notify("sys.heartbeat",std::vector<uint8_t>(strAck.begin(), strAck.end()));
                continue;
            }

            if(p.type == PacketType::Kick) {
                std::cout << "RecvThread received kick\n";
                break; 
            }

            if (p.type != PacketType::Data) continue;
            Message msg = MessageCodec::Decode(p.data);            
            // Response -> 唤醒 promise
            if (msg.type == MessageType::Response) {
                std::promise<Message> prom;
                {
                    std::lock_guard<std::mutex> lk(pending_mtx_);
                    auto it = pending_.find(static_cast<uint32_t>(msg.id));
                    if (it != pending_.end()) {
                        prom = std::move(it->second);
                        pending_.erase(it);
                    }
                }
                prom.set_value(std::move(msg));
                continue;
            }

            // 普通消息 -> 用户回调
            if (on_message_) on_message_(msg);
        }
    }
    Stop();
}

// ---------------- 其余工具 ----------------
uint32_t Client::NextMsgID() {
    std::lock_guard<std::mutex> lk(pending_mtx_);
    return ++id_seq_;
}

// ---------------- private ----------------
bool Client::sendHandshakeRequest() {
    auto json = defaultHandshake();
    std::string body = json.dump(); // 使用格式化输出
    
    // 确保JSON格式正确
    std::vector<uint8_t> pkt;
    try {
        pkt = Codec::Encode(PacketType::Handshake,
                                  std::vector<uint8_t>(body.begin(), body.end()));
        if (pkt.empty()) {
            return false;
        }
    } catch (const std::exception& e) {
        return false;
    }

    ssize_t n = ::write(sock_, pkt.data(), pkt.size());
    if (n != static_cast<ssize_t>(pkt.size())) {
        return false;
    }
    return true;
}

bool Client::handleHandshakeResponse() {
    std::vector<uint8_t> buffer;
    std::vector<Packet> pkts;
    if (!readPackets(buffer, pkts)) {
        return false;
    }

    if (pkts.empty()) {
        return false;
    }

    if (pkts[0].type != PacketType::Handshake) {
        return false;
    }

    auto& pkt = pkts[0];
    std::vector<uint8_t> data = pkt.data;

    HandshakeData hs = nlohmann::json::parse(std::string(data.begin(), data.end()));

    // 注册 route 字典
    for (auto& [route, code] : hs.sys.dict) {
        MessageCodec::RegisterRoute(route, code);
    }
    //回复 HandshakeAck
    std::string strAck = "{}";
    auto ack = Codec::Encode(PacketType::HandshakeAck, std::vector<uint8_t>(strAck.begin(), strAck.end()));
    ::write(sock_, ack.data(), ack.size());

    return true;
}

bool Client::readPackets(std::vector<uint8_t>& buffer, std::vector<Packet>& out) {
    ssize_t n = recvSome(buffer);
    if (n <= 0) return false;

    size_t consumed = 0;
    out = Codec::Decode(buffer, consumed);

    if (consumed > 0) {
        buffer.erase(buffer.begin(), buffer.begin() + consumed);
    }
    return !out.empty();
}

ssize_t Client::recvSome(std::vector<uint8_t>& buffer) {
    constexpr size_t kChunk = 4096;
    size_t old = buffer.size();
    buffer.resize(old + kChunk);

    ssize_t n = ::read(sock_, buffer.data() + old, kChunk);
    if (n < 0) {
        buffer.resize(old);
        return -1;
    }
    buffer.resize(old + n);
    return n;
}

} // namespace pitaya