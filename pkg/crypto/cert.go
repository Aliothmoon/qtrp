package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

// CertManager 证书管理器
type CertManager struct {
	cert    *tls.Certificate
	tlsConf *tls.Config
}

// CertConfig 证书配置
type CertConfig struct {
	// 证书文件路径（可选，为空则自动生成）
	CertFile string
	KeyFile  string

	// 自动生成证书的参数
	Organization string        // 组织名称
	CommonName   string        // 通用名称
	ValidFor     time.Duration // 有效期
	Hosts        []string      // 主机名/IP列表
}

// NewCertManager 创建证书管理器
func NewCertManager(cfg *CertConfig) (*CertManager, error) {
	cm := &CertManager{}

	var cert tls.Certificate
	var err error

	// 尝试加载已有证书
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err = tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err == nil {
			cm.cert = &cert
			cm.buildTLSConfig()
			return cm, nil
		}
		// 加载失败，生成新证书
	}

	// 生成自签名证书
	cert, err = cm.generateSelfSignedCert(cfg)
	if err != nil {
		return nil, fmt.Errorf("generate certificate: %w", err)
	}

	cm.cert = &cert
	cm.buildTLSConfig()

	// 如果指定了文件路径，保存证书
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		if err := cm.saveCert(cfg.CertFile, cfg.KeyFile); err != nil {
			// 保存失败只记录警告，不影响使用
			fmt.Printf("[cert] warning: save certificate failed: %v\n", err)
		}
	}

	return cm, nil
}

// generateSelfSignedCert 生成自签名证书
func (cm *CertManager) generateSelfSignedCert(cfg *CertConfig) (tls.Certificate, error) {
	// 使用 ECDSA P-256（比 RSA 更快）
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	// 设置默认值
	org := cfg.Organization
	if org == "" {
		org = "QTRP"
	}
	cn := cfg.CommonName
	if cn == "" {
		cn = "qtrp-server"
	}
	validFor := cfg.ValidFor
	if validFor == 0 {
		validFor = 365 * 24 * time.Hour // 1年
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
			CommonName:   cn,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// 添加主机名/IP
	hosts := cfg.Hosts
	if len(hosts) == 0 {
		hosts = []string{"localhost", "127.0.0.1", "::1"}
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	// 编码为 PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	return tls.X509KeyPair(certPEM, keyPEM)
}

// saveCert 保存证书到文件
func (cm *CertManager) saveCert(certFile, keyFile string) error {
	if cm.cert == nil || len(cm.cert.Certificate) == 0 {
		return fmt.Errorf("no certificate to save")
	}

	// 保存证书
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cm.cert.Certificate[0]})
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		return fmt.Errorf("write cert file: %w", err)
	}

	// 保存私钥
	privKey, ok := cm.cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("unsupported private key type")
	}
	privBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	return nil
}

// buildTLSConfig 构建 TLS 配置
func (cm *CertManager) buildTLSConfig() {
	cm.tlsConf = &tls.Config{
		Certificates: []tls.Certificate{*cm.cert},
		NextProtos:   []string{"qtrp"},
		MinVersion:   tls.VersionTLS13,
	}
}

// GetServerTLSConfig 获取服务端 TLS 配置
func (cm *CertManager) GetServerTLSConfig() *tls.Config {
	return cm.tlsConf
}

// GetClientTLSConfig 获取客户端 TLS 配置
func (cm *CertManager) GetClientTLSConfig(skipVerify bool) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: skipVerify,
		NextProtos:         []string{"qtrp"},
		MinVersion:         tls.VersionTLS13,
	}
}

// GetCertificate 获取证书
func (cm *CertManager) GetCertificate() *tls.Certificate {
	return cm.cert
}
