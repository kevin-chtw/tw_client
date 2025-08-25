#include "message.h"
#include <algorithm>
#include <stdexcept>
#include <cstdint>
#include <cstring>

namespace pitaya
{

    std::unordered_map<std::string, uint16_t> MessageCodec::s_routes;
    std::unordered_map<uint16_t, std::string> MessageCodec::s_codes;
    std::mutex MessageCodec::s_mu;

    /* static */
    bool MessageCodec::Routable(MessageType t)
    {
        return t == MessageType::Request || t == MessageType::Notify || t == MessageType::Push;
    }

    /* static */
    bool MessageCodec::InvalidType(MessageType t)
    {
        return static_cast<uint8_t>(t) < static_cast<uint8_t>(MessageType::Request) ||
               static_cast<uint8_t>(t) > static_cast<uint8_t>(MessageType::Push);
    }

    /* static */
    std::vector<uint8_t> MessageCodec::Encode(const Message &msg)
    {
        if (InvalidType(msg.type))
            throw std::invalid_argument("wrong message type");

        std::vector<uint8_t> buf;
        buf.reserve(64 + msg.data.size());

        uint8_t flag = (static_cast<uint8_t>(msg.type) << 1);

        uint16_t code = 0;
        bool routeCompressed = false;

        {
            std::lock_guard<std::mutex> lock(s_mu);
            auto it = s_routes.find(msg.route);
            if (it != s_routes.end())
            {
                code = it->second;
                routeCompressed = true;
            }
        }

        if (routeCompressed)
            flag |= kRouteCompressMask;
        if (msg.err)
            flag |= kErrorMask;

        buf.push_back(flag);

        // Encode ID (variable length)
        if (msg.type == MessageType::Request || msg.type == MessageType::Response)
        {
            uint32_t n = msg.id;
            do
            {
                uint8_t b = n & 0x7F;
                n >>= 7;
                if (n)
                    b |= 0x80;
                buf.push_back(b);
            } while (n);
        }

        // Encode Route
        if (Routable(msg.type))
        {
            if (routeCompressed)
            {
                buf.push_back(static_cast<uint8_t>((code >> 8) & 0xFF));
                buf.push_back(static_cast<uint8_t>(code & 0xFF));
            }
            else
            {
                if (msg.route.size() > 0xFF)
                    throw std::invalid_argument("route too long");
                buf.push_back(static_cast<uint8_t>(msg.route.size()));
                buf.insert(buf.end(), msg.route.begin(), msg.route.end());
            }
        }

        buf.insert(buf.end(), msg.data.begin(), msg.data.end());
        return buf;
    }

    /* static */
    Message MessageCodec::Decode(const std::vector<uint8_t> &data)
    {
        if (data.size() < kMsgHeadLength)
            throw std::invalid_argument("invalid message");

        Message msg;
        size_t offset = 1;

        uint8_t flag = data[0];
        msg.type = static_cast<MessageType>((flag >> 1) & kTypeMask);

        if (InvalidType(msg.type))
            throw std::invalid_argument("wrong message type");

        // Decode ID
        if (msg.type == MessageType::Request || msg.type == MessageType::Response)
        {
            uint32_t id = 0;
            int shift = 0;
            while (offset < data.size())
            {
                uint8_t b = data[offset++];
                id |= static_cast<uint32_t>(b & 0x7F) << shift;
                if ((b & 0x80) == 0)
                    break;
                shift += 7;
                if (shift > 28)
                    throw std::invalid_argument("id too large");
            }
            msg.id = id;
        }

        msg.err = (flag & kErrorMask) != 0;

        // Decode Route
        if (Routable(msg.type))
        {
            if (flag & kRouteCompressMask)
            {
                if (offset + 2 > data.size())
                    throw std::invalid_argument("invalid message: missing compressed route");

                uint16_t code = (static_cast<uint16_t>(data[offset]) << 8) | data[offset + 1];
                offset += 2;

                std::lock_guard<std::mutex> lock(s_mu);
                auto it = s_codes.find(code);
                if (it == s_codes.end())
                    throw std::runtime_error("route info not found in dictionary");

                msg.route = it->second;
                msg.compressed = true;
            }
            else
            {
                if (offset >= data.size())
                    throw std::invalid_argument("invalid message: missing route length");

                uint8_t len = data[offset++];
                if (offset + len > data.size())
                    throw std::invalid_argument("invalid message: route overflow");

                msg.route.assign(reinterpret_cast<const char *>(&data[offset]), len);
                offset += len;
                msg.compressed = false;
            }
        }

        if (offset > data.size())
            throw std::invalid_argument("invalid message");

        msg.data.assign(data.begin() + offset, data.end());

        // Decompress if gzip (temporarily disabled)
        if ((flag & kGzipMask) == kGzipMask)
        {
            // msg.data = compression::InflateData(msg.data);
        }

        return msg;
    }

    /* static */
    void MessageCodec::RegisterRoute(const std::string &route, uint16_t code)
    {
        std::lock_guard<std::mutex> lock(s_mu);
        s_routes[route] = code;
        s_codes[code] = route;
    }

    /* static */
    void MessageCodec::UnregisterRoute(const std::string &route)
    {
        std::lock_guard<std::mutex> lock(s_mu);
        auto it = s_routes.find(route);
        if (it != s_routes.end())
        {
            s_codes.erase(it->second);
            s_routes.erase(it);
        }
    }

    /* static */
    std::string MessageCodec::RouteByCode(uint16_t code)
    {
        std::lock_guard<std::mutex> lock(s_mu);
        auto it = s_codes.find(code);
        return (it != s_codes.end()) ? it->second : "";
    }

    /* static */
    uint16_t MessageCodec::CodeByRoute(const std::string &route)
    {
        std::lock_guard<std::mutex> lock(s_mu);
        auto it = s_routes.find(route);
        return (it != s_routes.end()) ? it->second : 0;
    }

} // namespace pitaya::message