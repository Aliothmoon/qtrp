package test

import (
	"strconv"
	"testing"
	"time"

	"qtrp/pkg/auth"
)

// TestAuthenticator 测试认证器
func TestAuthenticator(t *testing.T) {
	tokens := []string{"token1", "token2", "token3"}
	authenticator := auth.NewAuthenticator(tokens)

	t.Run("valid token", func(t *testing.T) {
		token := "token1"
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := auth.GenerateNonce()
		sign := auth.Sign(token, ts, nonce)

		if !authenticator.Verify(token, ts, nonce, sign) {
			t.Error("valid token should pass verification")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		token := "invalid-token"
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := auth.GenerateNonce()
		sign := auth.Sign(token, ts, nonce)

		if authenticator.Verify(token, ts, nonce, sign) {
			t.Error("invalid token should fail verification")
		}
	})

	t.Run("expired timestamp", func(t *testing.T) {
		token := "token1"
		ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10) // 10分钟前
		nonce := auth.GenerateNonce()
		sign := auth.Sign(token, ts, nonce)

		if authenticator.Verify(token, ts, nonce, sign) {
			t.Error("expired timestamp should fail verification")
		}
	})

	t.Run("future timestamp", func(t *testing.T) {
		token := "token1"
		ts := strconv.FormatInt(time.Now().Add(10*time.Minute).Unix(), 10) // 10分钟后
		nonce := auth.GenerateNonce()
		sign := auth.Sign(token, ts, nonce)

		if authenticator.Verify(token, ts, nonce, sign) {
			t.Error("future timestamp should fail verification")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		token := "token1"
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := auth.GenerateNonce()
		sign := "invalid-signature"

		if authenticator.Verify(token, ts, nonce, sign) {
			t.Error("invalid signature should fail verification")
		}
	})

	t.Run("replay attack (same nonce)", func(t *testing.T) {
		token := "token2"
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := auth.GenerateNonce()
		sign := auth.Sign(token, ts, nonce)

		// 第一次验证应该通过
		if !authenticator.Verify(token, ts, nonce, sign) {
			t.Error("first verification should pass")
		}

		// 重放攻击应该失败
		if authenticator.Verify(token, ts, nonce, sign) {
			t.Error("replay attack should fail (same nonce)")
		}
	})
}

// TestSign 测试签名生成
func TestSign(t *testing.T) {
	token := "test-token"
	ts := "1234567890"
	nonce := "random-nonce"

	sign1 := auth.Sign(token, ts, nonce)
	sign2 := auth.Sign(token, ts, nonce)

	// 相同输入应该产生相同签名
	if sign1 != sign2 {
		t.Error("same input should produce same signature")
	}

	// 不同输入应该产生不同签名
	sign3 := auth.Sign(token, ts, "different-nonce")
	if sign1 == sign3 {
		t.Error("different input should produce different signature")
	}

	// 签名应该是64位十六进制字符串 (SHA256)
	if len(sign1) != 64 {
		t.Errorf("signature length should be 64, got %d", len(sign1))
	}
}

// TestGenerateNonce 测试随机数生成
func TestGenerateNonce(t *testing.T) {
	nonces := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		nonce := auth.GenerateNonce()
		if nonces[nonce] {
			t.Errorf("duplicate nonce generated: %s", nonce)
		}
		nonces[nonce] = true

		// nonce 应该是32位十六进制字符串 (16 bytes)
		if len(nonce) != 32 {
			t.Errorf("nonce length should be 32, got %d", len(nonce))
		}
	}

	t.Logf("Generated %d unique nonces", count)
}
