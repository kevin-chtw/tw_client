#include "packet.h"
#include <algorithm>
#include <stdexcept>

namespace pitaya
{

    std::vector<uint8_t> Codec::Encode(PacketType type, const std::vector<uint8_t> &data)
    {
        uint8_t t = static_cast<uint8_t>(type);
        if (t < static_cast<uint8_t>(PacketType::Handshake) || t > static_cast<uint8_t>(PacketType::Kick))
        {
            throw std::invalid_argument("wrong packet type");
        }
        if (data.size() > kMaxPacketSize)
            throw std::length_error("packet size exceed");

        std::vector<uint8_t> buf;
        buf.reserve(kHeadLength + data.size());

        buf.push_back(t);        // type(1 byte)
        buf.resize(kHeadLength); // 先占位
        IntToBytes(buf.data() + 1, static_cast<uint32_t>(data.size()));

        buf.insert(buf.end(), data.begin(), data.end());
        return buf;
    }

    std::vector<Packet> Codec::Decode(const std::vector<uint8_t> &data, size_t &consumed)
    {
        std::vector<Packet> packets;
        size_t pos = 0;

        while (pos + kHeadLength <= data.size())
        {
            uint8_t typeByte = data[pos];
            uint32_t length = BytesToInt(data.data() + pos + 1);
            if (length > kMaxPacketSize)
                throw std::length_error("packet size exceed");

            if (pos + kHeadLength + length > data.size())
                break; // 数据不足，等待更多字节

            Packet p;
            p.type = static_cast<PacketType>(typeByte);
            p.length = length;
            p.data.assign(data.begin() + pos + kHeadLength,
                          data.begin() + pos + kHeadLength + length);
            packets.emplace_back(std::move(p));

            pos += kHeadLength + length;
        }

        consumed = pos;
        return packets;
    }

    /* static */
    uint32_t Codec::BytesToInt(const uint8_t *b)
    {
        return (static_cast<uint32_t>(b[0]) << 16) |
               (static_cast<uint32_t>(b[1]) << 8) |
               static_cast<uint32_t>(b[2]);
    }

    /* static */
    void Codec::IntToBytes(uint8_t *buf, uint32_t n)
    {
        buf[0] = static_cast<uint8_t>((n >> 16) & 0xFF);
        buf[1] = static_cast<uint8_t>((n >> 8) & 0xFF);
        buf[2] = static_cast<uint8_t>(n & 0xFF);
    }

} // namespace pitaya::packet