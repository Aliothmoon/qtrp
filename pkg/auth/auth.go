package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"
)

const (
	// NonceExpiry nonce 过期时间
	NonceExpiry = 5 * time.Minute
	// MaxTimeDiff 允许的最大时间差
	MaxTimeDiff = 5 * time.Minute
)

// Authenticator 认证器
type Authenticator struct {
	tokens     map[string]bool
	nonceCache *NonceCache
	mu         sync.RWMutex
}

// NewAuthenticator 创建认证器
func NewAuthenticator(tokens []string) *Authenticator {
	tokenMap := make(map[string]bool)
	for _, t := range tokens {
		tokenMap[t] = true
	}
	return &Authenticator{
		tokens:     tokenMap,
		nonceCache: NewNonceCache(NonceExpiry),
	}
}

// Verify 验证认证请求
func (a *Authenticator) Verify(token, timestamp, nonce, sign string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 1. 验证 token 是否存在
	if !a.tokens[token] {
		return false
	}

	// 2. 验证时间戳
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	if abs(now-ts) > int64(MaxTimeDiff.Seconds()) {
		return false
	}

	// 3. 验证 nonce 是否重复
	if a.nonceCache.Exists(nonce) {
		return false
	}
	a.nonceCache.Add(nonce)

	// 4. 验证签名
	expectedSign := Sign(token, timestamp, nonce)
	return hmac.Equal([]byte(sign), []byte(expectedSign))
}

// Sign 生成签名
func Sign(token, timestamp, nonce string) string {
	data := token + timestamp + nonce
	h := hmac.New(sha256.New, []byte(token))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateNonce 生成随机 nonce
func GenerateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// NonceCache nonce 缓存
type NonceCache struct {
	cache  map[string]time.Time
	expiry time.Duration
	mu     sync.RWMutex
}

// NewNonceCache 创建 nonce 缓存
func NewNonceCache(expiry time.Duration) *NonceCache {
	nc := &NonceCache{
		cache:  make(map[string]time.Time),
		expiry: expiry,
	}
	go nc.cleanup()
	return nc
}

// Add 添加 nonce
func (nc *NonceCache) Add(nonce string) {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.cache[nonce] = time.Now()
}

// Exists 检查 nonce 是否存在
func (nc *NonceCache) Exists(nonce string) bool {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	_, exists := nc.cache[nonce]
	return exists
}

// cleanup 定期清理过期 nonce
func (nc *NonceCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		nc.mu.Lock()
		now := time.Now()
		for nonce, t := range nc.cache {
			if now.Sub(t) > nc.expiry {
				delete(nc.cache, nonce)
			}
		}
		nc.mu.Unlock()
	}
}
