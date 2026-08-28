// Reference order book server — correct (price-time priority) but deliberately
// simple, not fast. Self-contained: only the C++ standard library and POSIX
// sockets, so it builds with `g++ -O2 -std=c++17 -o orderbook main.cpp`.
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <algorithm>
#include <cstring>
#include <deque>
#include <map>
#include <mutex>
#include <sstream>
#include <string>
#include <thread>
#include <unordered_map>

// ----- minimal JSON helpers (good enough for the fixed request shapes) -----
static std::string jsonStr(const std::string& body, const std::string& key) {
    auto pos = body.find("\"" + key + "\"");
    if (pos == std::string::npos) return "";
    pos = body.find(':', pos);
    if (pos == std::string::npos) return "";
    pos = body.find('"', pos);
    if (pos == std::string::npos) return "";
    auto end = body.find('"', pos + 1);
    return body.substr(pos + 1, end - pos - 1);
}
static double jsonNum(const std::string& body, const std::string& key, double def = 0) {
    auto pos = body.find("\"" + key + "\"");
    if (pos == std::string::npos) return def;
    pos = body.find(':', pos);
    if (pos == std::string::npos) return def;
    ++pos;
    while (pos < body.size() && (body[pos] == ' ' || body[pos] == '"')) ++pos;
    try {
        return std::stod(body.substr(pos));
    } catch (...) {
        return def;
    }
}

// ----- order book -----
struct Order {
    std::string id;
    double price;
    double qty;
};

struct Book {
    std::mutex mu;
    std::map<double, std::deque<Order>, std::greater<double>> bids;  // high first
    std::map<double, std::deque<Order>> asks;                        // low first
    std::unordered_map<std::string, std::pair<bool, double>> lookup; // id -> (isBuy, price)

    std::string handleOrder(const std::string& body) {
        std::lock_guard<std::mutex> lk(mu);
        std::string id = jsonStr(body, "order_id");
        std::string type = jsonStr(body, "type");
        double price = jsonNum(body, "price");
        double qty = jsonNum(body, "quantity");

        double filled = 0, lastPx = 0, remaining = qty;
        std::string status;

        if (type == "MARKET_BUY" || type == "LIMIT_BUY") {
            while (remaining > 0 && !asks.empty()) {
                auto it = asks.begin();
                if (type == "LIMIT_BUY" && it->first > price) break;
                auto& q = it->second;
                while (remaining > 0 && !q.empty()) {
                    double m = std::min(remaining, q.front().qty);
                    filled += m; lastPx = it->first; remaining -= m; q.front().qty -= m;
                    if (q.front().qty <= 0) { lookup.erase(q.front().id); q.pop_front(); }
                }
                if (q.empty()) asks.erase(it);
            }
            if (type == "LIMIT_BUY" && remaining > 0) {
                bids[price].push_back({id, price, remaining});
                lookup[id] = {true, price};
            }
        } else if (type == "MARKET_SELL" || type == "LIMIT_SELL") {
            while (remaining > 0 && !bids.empty()) {
                auto it = bids.begin();
                if (type == "LIMIT_SELL" && it->first < price) break;
                auto& q = it->second;
                while (remaining > 0 && !q.empty()) {
                    double m = std::min(remaining, q.front().qty);
                    filled += m; lastPx = it->first; remaining -= m; q.front().qty -= m;
                    if (q.front().qty <= 0) { lookup.erase(q.front().id); q.pop_front(); }
                }
                if (q.empty()) bids.erase(it);
            }
            if (type == "LIMIT_SELL" && remaining > 0) {
                asks[price].push_back({id, price, remaining});
                lookup[id] = {false, price};
            }
        }

        if (filled == 0 && (type == "MARKET_BUY" || type == "MARKET_SELL"))
            status = "REJECTED";
        else if (filled == 0)
            status = "PENDING";
        else if (remaining > 0 && (type == "LIMIT_BUY" || type == "LIMIT_SELL"))
            status = "PARTIAL";
        else if (remaining > 0)
            status = "PARTIAL";
        else
            status = "FILLED";

        std::ostringstream os;
        os << "{\"order_id\":\"" << id << "\",\"status\":\"" << status
           << "\",\"filled_price\":" << lastPx
           << ",\"filled_quantity\":" << filled
           << ",\"remaining_quantity\":" << remaining << "}";
        return os.str();
    }

    std::string handleCancel(const std::string& body) {
        std::lock_guard<std::mutex> lk(mu);
        std::string id = jsonStr(body, "order_id");
        auto it = lookup.find(id);
        if (it == lookup.end())
            return "{\"order_id\":\"" + id + "\",\"status\":\"NOT_FOUND\"}";
        double px = it->second.second;
        if (it->second.first) {
            auto lvl = bids.find(px);
            if (lvl != bids.end()) {
                auto& q = lvl->second;
                for (auto qit = q.begin(); qit != q.end(); ++qit)
                    if (qit->id == id) { q.erase(qit); break; }
                if (q.empty()) bids.erase(lvl);
            }
        } else {
            auto lvl = asks.find(px);
            if (lvl != asks.end()) {
                auto& q = lvl->second;
                for (auto qit = q.begin(); qit != q.end(); ++qit)
                    if (qit->id == id) { q.erase(qit); break; }
                if (q.empty()) asks.erase(lvl);
            }
        }
        lookup.erase(it);
        return "{\"order_id\":\"" + id + "\",\"status\":\"CANCELLED\"}";
    }

    void reset() {
        std::lock_guard<std::mutex> lk(mu);
        bids.clear(); asks.clear(); lookup.clear();
    }
};

static Book book;

static void respond(int fd, const std::string& payload) {
    std::ostringstream os;
    os << "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: "
       << payload.size() << "\r\nConnection: close\r\n\r\n" << payload;
    auto s = os.str();
    (void)write(fd, s.data(), s.size());
}

static void handle(int fd) {
    char buf[8192];
    ssize_t n = read(fd, buf, sizeof(buf) - 1);
    if (n <= 0) { close(fd); return; }
    buf[n] = 0;
    std::string req(buf, n);

    std::string method = req.substr(0, req.find(' '));
    auto pathStart = req.find(' ') + 1;
    std::string path = req.substr(pathStart, req.find(' ', pathStart) - pathStart);
    std::string body;
    auto bpos = req.find("\r\n\r\n");
    if (bpos != std::string::npos) body = req.substr(bpos + 4);

    if (path == "/health") {
        respond(fd, "{\"status\":\"ok\"}");
    } else if (method == "POST" && path == "/order") {
        respond(fd, book.handleOrder(body));
    } else if (method == "POST" && path == "/cancel") {
        respond(fd, book.handleCancel(body));
    } else if (method == "POST" && path == "/reset") {
        book.reset();
        respond(fd, "{\"status\":\"reset\"}");
    } else {
        respond(fd, "{\"error\":\"not found\"}");
    }
    close(fd);
}

int main() {
    int srv = socket(AF_INET, SOCK_STREAM, 0);
    int opt = 1;
    setsockopt(srv, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons(8080);
    if (bind(srv, (sockaddr*)&addr, sizeof(addr)) < 0) return 1;
    listen(srv, 512);
    for (;;) {
        int fd = accept(srv, nullptr, nullptr);
        if (fd < 0) continue;
        std::thread(handle, fd).detach();
    }
}
