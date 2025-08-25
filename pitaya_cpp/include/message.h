#pragma once
#include <cstdint>
#include <string>
#include <vector>
#include <unordered_map>
#include <mutex>
#include <stdexcept>

namespace pitaya
{

    enum class MessageType : uint8_t
    {
        Request = 0x00,
        Notify = 0x01,
        Response = 0x02,
        Push = 0x03,
    };

    constexpr uint8_t kErrorMask = 0x20;
    constexpr uint8_t kGzipMask = 0x10;
    constexpr uint8_t kRouteCompressMask = 0x01;
    constexpr uint8_t kTypeMask = 0x07;
    constexpr uint8_t kRouteLengthMask = 0xFF;
    constexpr size_t kMsgHeadLength = 2;

    struct Message
    {
        MessageType type;
        uint32_t id{0}; // 与 Go 侧 uint 对应，实际 32 位足够
        std::string route;
        std::vector<uint8_t> data;
        bool compressed{false};
        bool err{false};
    };

    class MessageCodec
    {
    public:
        static std::vector<uint8_t> Encode(const Message &msg);
        static Message Decode(const std::vector<uint8_t> &data);

        // 全局字典操作（线程安全）
        static void RegisterRoute(const std::string &route, uint16_t code);
        static void UnregisterRoute(const std::string &route);
        static std::string RouteByCode(uint16_t code);
        static uint16_t CodeByRoute(const std::string &route);

    private:
        static bool Routable(MessageType t);
        static bool InvalidType(MessageType t);

        static std::unordered_map<std::string, uint16_t> s_routes;
        static std::unordered_map<uint16_t, std::string> s_codes;
        static std::mutex s_mu;
    };

} // namespace message