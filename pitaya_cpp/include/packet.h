#pragma once
#include <cstdint>
#include <vector>
#include <string>
#include <stdexcept>

namespace pitaya {

enum class PacketType : uint8_t {
    // 0x00 被 iota 跳过
    Handshake    = 0x01,
    HandshakeAck = 0x02,
    Heartbeat    = 0x03,
    Data         = 0x04,
    Kick         = 0x05
};

constexpr size_t kHeadLength    = 4;
constexpr size_t kMaxPacketSize = 1 << 24;   // 16 MB

struct Packet {
    PacketType type;
    uint32_t   length;   // 有效负载长度
    std::vector<uint8_t> data;
};

class Codec {
public:
    /* 编码一个包 */
    static std::vector<uint8_t> Encode(PacketType type,
                                       const std::vector<uint8_t>& data);

    /* 从字节流中解出一个或多个包 */
    static std::vector<Packet> Decode(const std::vector<uint8_t>& data,
                                      size_t& consumed);   // 返回已消费字节数

private:
    static uint32_t BytesToInt(const uint8_t* b);
    static void     IntToBytes(uint8_t* buf, uint32_t n);
};

} // namespace pitaya