package test

import (
	"testing"

	"qtrp/pkg/config"
)

// TestParsePortRange 测试端口范围解析
func TestParsePortRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		start   int
		end     int
		wantErr bool
	}{
		{"single port", "8080", 8080, 8080, false},
		{"port range", "8000-8010", 8000, 8010, false},
		{"same port range", "8080-8080", 8080, 8080, false},
		{"min port", "1", 1, 1, false},
		{"max port", "65535", 65535, 65535, false},
		{"full range", "1-65535", 1, 65535, false},

		// 错误情况
		{"empty", "", 0, 0, true},
		{"invalid", "abc", 0, 0, true},
		{"negative", "-1", 0, 0, true},
		{"zero", "0", 0, 0, true},
		{"too large", "65536", 0, 0, true},
		{"reversed range", "8010-8000", 0, 0, true},
		{"invalid format", "8000-8010-8020", 0, 0, true},
		{"invalid start", "abc-8010", 0, 0, true},
		{"invalid end", "8000-abc", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr, err := config.ParsePortRange(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}

			if pr.Start != tt.start || pr.End != tt.end {
				t.Errorf("expected range %d-%d, got %d-%d", tt.start, tt.end, pr.Start, pr.End)
			}
		})
	}
}

// TestPortRangeCount 测试端口数量计算
func TestPortRangeCount(t *testing.T) {
	tests := []struct {
		input string
		count int
	}{
		{"8080", 1},
		{"8000-8000", 1},
		{"8000-8001", 2},
		{"8000-8010", 11},
		{"1-100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pr, _ := config.ParsePortRange(tt.input)
			if pr.Count() != tt.count {
				t.Errorf("expected count %d, got %d", tt.count, pr.Count())
			}
		})
	}
}

// TestPortRangePorts 测试端口列表生成
func TestPortRangePorts(t *testing.T) {
	pr, _ := config.ParsePortRange("8000-8005")
	ports := pr.Ports()

	expected := []int{8000, 8001, 8002, 8003, 8004, 8005}
	if len(ports) != len(expected) {
		t.Fatalf("expected %d ports, got %d", len(expected), len(ports))
	}

	for i, p := range ports {
		if p != expected[i] {
			t.Errorf("port %d: expected %d, got %d", i, expected[i], p)
		}
	}
}

// TestProxyConfigExpand 测试代理配置展开
func TestProxyConfigExpand(t *testing.T) {
	t.Run("single port", func(t *testing.T) {
		cfg := &config.ProxyConfig{
			Name:       "test",
			Type:       "tcp",
			LocalAddr:  "127.0.0.1",
			LocalPort:  "8080",
			RemotePort: "6080",
		}

		expanded, err := cfg.Expand()
		if err != nil {
			t.Fatalf("Expand failed: %v", err)
		}

		if len(expanded) != 1 {
			t.Fatalf("expected 1 proxy, got %d", len(expanded))
		}

		ep := expanded[0]
		if ep.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", ep.Name)
		}
		if ep.LocalPort != 8080 {
			t.Errorf("expected local port 8080, got %d", ep.LocalPort)
		}
		if ep.RemotePort != 6080 {
			t.Errorf("expected remote port 6080, got %d", ep.RemotePort)
		}
		if ep.GetLocalAddress() != "127.0.0.1:8080" {
			t.Errorf("expected address '127.0.0.1:8080', got '%s'", ep.GetLocalAddress())
		}
	})

	t.Run("port range", func(t *testing.T) {
		cfg := &config.ProxyConfig{
			Name:       "range",
			Type:       "tcp",
			LocalAddr:  "192.168.1.1",
			LocalPort:  "8000-8002",
			RemotePort: "6000-6002",
		}

		expanded, err := cfg.Expand()
		if err != nil {
			t.Fatalf("Expand failed: %v", err)
		}

		if len(expanded) != 3 {
			t.Fatalf("expected 3 proxies, got %d", len(expanded))
		}

		expectedNames := []string{"range_0", "range_1", "range_2"}
		expectedLocal := []int{8000, 8001, 8002}
		expectedRemote := []int{6000, 6001, 6002}

		for i, ep := range expanded {
			if ep.Name != expectedNames[i] {
				t.Errorf("proxy %d: expected name '%s', got '%s'", i, expectedNames[i], ep.Name)
			}
			if ep.LocalPort != expectedLocal[i] {
				t.Errorf("proxy %d: expected local port %d, got %d", i, expectedLocal[i], ep.LocalPort)
			}
			if ep.RemotePort != expectedRemote[i] {
				t.Errorf("proxy %d: expected remote port %d, got %d", i, expectedRemote[i], ep.RemotePort)
			}
		}
	})

	t.Run("mismatched range", func(t *testing.T) {
		cfg := &config.ProxyConfig{
			Name:       "mismatch",
			Type:       "tcp",
			LocalAddr:  "127.0.0.1",
			LocalPort:  "8000-8005", // 6 ports
			RemotePort: "6000-6002", // 3 ports
		}

		_, err := cfg.Expand()
		if err == nil {
			t.Error("expected error for mismatched port ranges")
		}
	})

	t.Run("invalid local port", func(t *testing.T) {
		cfg := &config.ProxyConfig{
			Name:       "invalid",
			Type:       "tcp",
			LocalAddr:  "127.0.0.1",
			LocalPort:  "invalid",
			RemotePort: "6000",
		}

		_, err := cfg.Expand()
		if err == nil {
			t.Error("expected error for invalid local port")
		}
	})
}
