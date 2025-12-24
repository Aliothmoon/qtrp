package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig 服务端配置
type ServerConfig struct {
	BindAddr      string `yaml:"bind_addr"`
	DashboardAddr string `yaml:"dashboard_addr"`
	Transport     string `yaml:"transport"` // "net" | "gnet" | "quic"
	Crypto        string `yaml:"crypto"`    // "tls" | "chacha20" | "none" (quic 模式自带加密)

	TLS struct {
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"tls"`

	ChaCha20 struct {
		PSK string `yaml:"psk"` // 预共享密钥 (32 bytes)
	} `yaml:"chacha20"`

	QUIC struct {
		CertFile string `yaml:"cert_file"` // 证书文件（可选，为空自动生成）
		KeyFile  string `yaml:"key_file"`  // 私钥文件（可选）
	} `yaml:"quic"`

	Auth struct {
		Tokens []string `yaml:"tokens"`
	} `yaml:"auth"`

	// MTMux 多隧道复用配置
	MTMux struct {
		Enabled bool `yaml:"enabled"` // 启用多隧道模式
		Tunnels int  `yaml:"tunnels"` // 并行连接数 (默认 8)
	} `yaml:"mtmux"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
	ServerAddr string `yaml:"server_addr"`
	Transport  string `yaml:"transport"` // "net" | "gnet" | "quic"
	Crypto     string `yaml:"crypto"`    // "tls" | "chacha20" | "none" (quic 模式自带加密)
	Token      string `yaml:"token"`

	TLS struct {
		CACert     string `yaml:"ca_cert"`
		SkipVerify bool   `yaml:"skip_verify"`
	} `yaml:"tls"`

	ChaCha20 struct {
		PSK string `yaml:"psk"`
	} `yaml:"chacha20"`

	QUIC struct {
		SkipVerify bool `yaml:"skip_verify"` // 跳过证书验证（自签名证书需要）
	} `yaml:"quic"`

	// MTMux 多隧道复用配置
	MTMux struct {
		Enabled bool `yaml:"enabled"` // 启用多隧道模式 (必须与服务端一致)
		Tunnels int  `yaml:"tunnels"` // 并行连接数 (必须与服务端一致, 默认 8)
	} `yaml:"mtmux"`

	Proxies []ProxyConfig `yaml:"proxies"`
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`        // "tcp"
	LocalAddr  string `yaml:"local_addr"`  // 本地 IP，如 "127.0.0.1"
	LocalPort  string `yaml:"local_port"`  // 本地端口，支持范围如 "8000" 或 "8000-8010"
	RemotePort string `yaml:"remote_port"` // 远程端口，支持范围如 "6000" 或 "6000-6010"
}

// PortRange 端口范围
type PortRange struct {
	Start int
	End   int
}

// ParsePortRange 解析端口范围字符串
// 支持格式: "8000" 或 "8000-8010"
func ParsePortRange(s string) (*PortRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty port string")
	}

	parts := strings.Split(s, "-")
	if len(parts) == 1 {
		port, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", s)
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port out of range: %d", port)
		}
		return &PortRange{Start: port, End: port}, nil
	}

	if len(parts) == 2 {
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid start port: %s", parts[0])
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid end port: %s", parts[1])
		}
		if start < 1 || start > 65535 || end < 1 || end > 65535 {
			return nil, fmt.Errorf("port out of range: %s", s)
		}
		if start > end {
			return nil, fmt.Errorf("start port > end port: %s", s)
		}
		return &PortRange{Start: start, End: end}, nil
	}

	return nil, fmt.Errorf("invalid port format: %s", s)
}

// Count 返回端口数量
func (r *PortRange) Count() int {
	return r.End - r.Start + 1
}

// Ports 返回所有端口列表
func (r *PortRange) Ports() []int {
	ports := make([]int, 0, r.Count())
	for p := r.Start; p <= r.End; p++ {
		ports = append(ports, p)
	}
	return ports
}

// ExpandedProxy 展开后的代理配置（单个端口）
type ExpandedProxy struct {
	Name       string
	Type       string
	LocalAddr  string
	LocalPort  int
	RemotePort int
}

// Expand 展开端口范围为多个代理配置
func (p *ProxyConfig) Expand() ([]*ExpandedProxy, error) {
	localRange, err := ParsePortRange(p.LocalPort)
	if err != nil {
		return nil, fmt.Errorf("proxy %s: invalid local_port: %w", p.Name, err)
	}

	remoteRange, err := ParsePortRange(p.RemotePort)
	if err != nil {
		return nil, fmt.Errorf("proxy %s: invalid remote_port: %w", p.Name, err)
	}

	if localRange.Count() != remoteRange.Count() {
		return nil, fmt.Errorf("proxy %s: local_port and remote_port range count mismatch (%d vs %d)",
			p.Name, localRange.Count(), remoteRange.Count())
	}

	localPorts := localRange.Ports()
	remotePorts := remoteRange.Ports()
	proxies := make([]*ExpandedProxy, len(localPorts))

	for i := 0; i < len(localPorts); i++ {
		name := p.Name
		if len(localPorts) > 1 {
			name = fmt.Sprintf("%s_%d", p.Name, i)
		}
		proxies[i] = &ExpandedProxy{
			Name:       name,
			Type:       p.Type,
			LocalAddr:  p.LocalAddr,
			LocalPort:  localPorts[i],
			RemotePort: remotePorts[i],
		}
	}

	return proxies, nil
}

// GetLocalAddress 返回完整的本地地址 "ip:port"
func (e *ExpandedProxy) GetLocalAddress() string {
	return fmt.Sprintf("%s:%d", e.LocalAddr, e.LocalPort)
}

// LoadServerConfig 加载服务端配置
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 默认值
	if cfg.Transport == "" {
		cfg.Transport = "net"
	}
	if cfg.Crypto == "" {
		cfg.Crypto = "tls"
	}

	// MTMux 默认值和验证
	if cfg.MTMux.Enabled {
		if cfg.MTMux.Tunnels <= 0 {
			cfg.MTMux.Tunnels = 8 // 默认 8 条隧道
		}
		if cfg.MTMux.Tunnels > 32 {
			return nil, fmt.Errorf("mtmux.tunnels cannot exceed 32 (got %d)", cfg.MTMux.Tunnels)
		}
		if cfg.Transport == "quic" {
			return nil, fmt.Errorf("mtmux cannot be used with transport=quic (QUIC has built-in multiplexing)")
		}
	}

	return &cfg, nil
}

// LoadClientConfig 加载客户端配置
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg ClientConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 默认值
	if cfg.Transport == "" {
		cfg.Transport = "net"
	}
	if cfg.Crypto == "" {
		cfg.Crypto = "tls"
	}

	// MTMux 默认值和验证
	if cfg.MTMux.Enabled {
		if cfg.MTMux.Tunnels <= 0 {
			cfg.MTMux.Tunnels = 8 // 默认 8 条隧道
		}
		if cfg.MTMux.Tunnels > 32 {
			return nil, fmt.Errorf("mtmux.tunnels cannot exceed 32 (got %d)", cfg.MTMux.Tunnels)
		}
		if cfg.Transport == "quic" {
			return nil, fmt.Errorf("mtmux cannot be used with transport=quic (QUIC has built-in multiplexing)")
		}
	}

	return &cfg, nil
}
