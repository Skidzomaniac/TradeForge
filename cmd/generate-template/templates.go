package main

// templates maps language -> filename -> contents. The C++ starter is the
// self-contained reference (no external headers), so it builds in the sandbox.
var templates = map[string]map[string]string{
	"cpp": {
		"main.cpp": cppMain,
		"README.md": "# C++ starter\n\nBuild: `g++ -O2 -std=c++17 -o orderbook main.cpp -lpthread`\nRun: `./orderbook` (listens on :8080)\n",
	},
	"go": {
		"main.go": goMain,
		"go.mod":  "module orderbook\n\ngo 1.22\n",
		"README.md": "# Go starter\n\nBuild: `go build -o orderbook ./...`\nRun: `./orderbook`\n",
	},
	"python": {
		"main.py":  pyMain,
		"README.md": "# Python starter\n\nRun: `python main.py` (listens on :8080)\n",
	},
	"rust": {
		"README.md": "# Rust starter\n\nImplement POST /order, POST /cancel, GET /health on :8080.\nSee the order book API contract in the platform README.\n",
	},
}

const goMain = `package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

type order struct {
	OrderID  string  ` + "`json:\"order_id\"`" + `
	Type     string  ` + "`json:\"type\"`" + `
	Price    float64 ` + "`json:\"price\"`" + `
	Quantity float64 ` + "`json:\"quantity\"`" + `
}

var mu sync.Mutex

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(` + "`{\"status\":\"ok\"}`" + `))
	})
	http.HandleFunc("/order", func(w http.ResponseWriter, r *http.Request) {
		var o order
		json.NewDecoder(r.Body).Decode(&o)
		mu.Lock(); defer mu.Unlock()
		// TODO: implement matching. This stub rests every order.
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": o.OrderID, "status": "PENDING",
			"filled_price": 0.0, "filled_quantity": 0.0, "remaining_quantity": o.Quantity,
		})
	})
	http.HandleFunc("/cancel", func(w http.ResponseWriter, r *http.Request) {
		var o order
		json.NewDecoder(r.Body).Decode(&o)
		json.NewEncoder(w).Encode(map[string]string{"order_id": o.OrderID, "status": "CANCELLED"})
	})
	http.ListenAndServe(":8080", nil)
}
`

const pyMain = `import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

class H(BaseHTTPRequestHandler):
    def _send(self, obj):
        body = json.dumps(obj).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self._send({"status": "ok"})

    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        data = json.loads(self.rfile.read(n) or b"{}")
        if self.path == "/order":
            # TODO: implement matching. This stub rests every order.
            self._send({"order_id": data.get("order_id"), "status": "PENDING",
                        "filled_price": 0, "filled_quantity": 0,
                        "remaining_quantity": data.get("quantity", 0)})
        elif self.path == "/cancel":
            self._send({"order_id": data.get("order_id"), "status": "CANCELLED"})

ThreadingHTTPServer(("0.0.0.0", 8080), H).serve_forever()
`

const cppMain = `// Minimal correct order book starter. Build:
//   g++ -O2 -std=c++17 -o orderbook main.cpp -lpthread
// See the order book API contract in the platform README. This stub rests every limit
// order and rejects market orders; replace with your matching engine.
#include <arpa/inet.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>
#include <string>
#include <thread>

static void respond(int fd, const std::string& p) {
    std::string s = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: " +
                    std::to_string(p.size()) + "\r\nConnection: close\r\n\r\n" + p;
    (void)write(fd, s.data(), s.size());
}
static void handle(int fd) {
    char buf[4096]; ssize_t n = read(fd, buf, sizeof(buf) - 1);
    if (n <= 0) { close(fd); return; }
    buf[n] = 0; std::string req(buf);
    if (req.find("GET /health") == 0) respond(fd, "{\"status\":\"ok\"}");
    else if (req.find("POST /order") == 0) respond(fd, "{\"status\":\"PENDING\",\"filled_quantity\":0,\"remaining_quantity\":0}");
    else if (req.find("POST /cancel") == 0) respond(fd, "{\"status\":\"CANCELLED\"}");
    else respond(fd, "{\"error\":\"not found\"}");
    close(fd);
}
int main() {
    int s = socket(AF_INET, SOCK_STREAM, 0); int opt = 1;
    setsockopt(s, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    sockaddr_in a{}; a.sin_family = AF_INET; a.sin_addr.s_addr = INADDR_ANY; a.sin_port = htons(8080);
    bind(s, (sockaddr*)&a, sizeof(a)); listen(s, 128);
    for (;;) { int fd = accept(s, nullptr, nullptr); if (fd >= 0) std::thread(handle, fd).detach(); }
}
`
