package server

import (
	"encoding/json"
	"log"
	"net/http"

	"qtrp/pkg/stats"
)

// startDashboard 启动 Dashboard HTTP 服务
func (s *Server) startDashboard(addr string) {
	mux := http.NewServeMux()

	// 统计接口
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats.Get())
	})

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// 代理列表
	mux.HandleFunc("/api/proxies", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var proxies []map[string]interface{}
		s.proxies.Range(func(key, value interface{}) bool {
			proxy := value.(*Proxy)
			proxies = append(proxies, map[string]interface{}{
				"name":        proxy.name,
				"remote_port": proxy.remotePort,
				"client_id":   proxy.client.id,
			})
			return true
		})
		json.NewEncoder(w).Encode(proxies)
	})

	// 客户端列表
	mux.HandleFunc("/api/clients", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var clients []map[string]interface{}
		s.clients.Range(func(key, value interface{}) bool {
			client := value.(*Client)
			client.mu.RLock()
			proxyCount := len(client.proxies)
			lastPing := client.lastPing
			client.mu.RUnlock()

			clients = append(clients, map[string]interface{}{
				"id":          client.id,
				"proxy_count": proxyCount,
				"last_ping":   lastPing,
				"remote_addr": client.session.RemoteAddr().String(),
			})
			return true
		})
		json.NewEncoder(w).Encode(clients)
	})

	log.Printf("[dashboard] listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("[dashboard] error: %v", err)
	}
}
