package encryption

import (
	"strings"
	"testing"
)

func TestMD5(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
		{"Hello World", "b10a8db164e0754105b7a99be72e3fe5"},
		{"test123", "cc03e747a6afbbcbf8be7668acfebee5"},
	}

	for _, tt := range tests {
		result := MD5(tt.input)
		if result != tt.expected {
			t.Errorf("MD5(%q) = %s, 期望 %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetMD5ByStr(t *testing.T) {
	// 测试 CasaOS 兼容别名
	result := GetMD5ByStr("hello")
	expected := "5d41402abc4b2a76b9719d911017c592"
	
	if result != expected {
		t.Errorf("GetMD5ByStr('hello') = %s, 期望 %s", result, expected)
	}
}

func TestSHA256(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}

	for _, tt := range tests {
		result := SHA256(tt.input)
		if result != tt.expected {
			t.Errorf("SHA256(%q) = %s, 期望 %s", tt.input, result, tt.expected)
		}
	}
}

func TestSHA512(t *testing.T) {
	result := SHA512("hello")
	
	// SHA512 结果应该是 128 个十六进制字符
	if len(result) != 128 {
		t.Errorf("SHA512 结果长度应为 128, 实际为 %d", len(result))
	}
}

func TestMD5Bytes(t *testing.T) {
	data := []byte("hello")
	result := MD5Bytes(data)
	expected := "5d41402abc4b2a76b9719d911017c592"
	
	if result != expected {
		t.Errorf("MD5Bytes(hello) = %s, 期望 %s", result, expected)
	}
}

func TestBcryptPassword(t *testing.T) {
	password := "mySecretPassword123"
	
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	
	// bcrypt hash 应该以 $2a$ 或 $2b$ 开头
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("bcrypt hash 格式不正确: %s", hash)
	}
	
	// 验证正确密码
	if !CheckPassword(password, hash) {
		t.Error("正确密码验证失败")
	}
	
	// 验证错误密码
	if CheckPassword("wrongPassword", hash) {
		t.Error("错误密码不应通过验证")
	}
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "aGVsbG8="},
		{"Hello World", "SGVsbG8gV29ybGQ="},
	}

	for _, tt := range tests {
		result := Base64Encode([]byte(tt.input))
		if result != tt.expected {
			t.Errorf("Base64Encode(%q) = %s, 期望 %s", tt.input, result, tt.expected)
		}
	}
}

func TestBase64Decode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"aGVsbG8=", "hello"},
		{"SGVsbG8gV29ybGQ=", "Hello World"},
	}

	for _, tt := range tests {
		result, err := Base64Decode(tt.input)
		if err != nil {
			t.Errorf("Base64Decode(%q) 返回错误: %v", tt.input, err)
			continue
		}
		if string(result) != tt.expected {
			t.Errorf("Base64Decode(%q) = %s, 期望 %s", tt.input, string(result), tt.expected)
		}
	}
}

func TestBase64URLEncode(t *testing.T) {
	// URL 安全的 Base64 不应包含 + 和 /
	data := []byte{0xfb, 0xef, 0xbe}
	result := Base64URLEncode(data)
	
	if strings.ContainsAny(result, "+/") {
		t.Errorf("Base64URLEncode 结果不应包含 + 或 /: %s", result)
	}
}

func TestAESEncryptDecrypt(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 字节 AES-128
	plaintext := "Hello, AES Encryption!"
	
	// 加密
	ciphertext, err := AESEncrypt([]byte(plaintext), key)
	if err != nil {
		t.Fatalf("AESEncrypt 失败: %v", err)
	}
	
	// 密文不应与明文相同
	if string(ciphertext) == plaintext {
		t.Error("密文不应与明文相同")
	}
	
	// 解密
	decrypted, err := AESDecrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("AESDecrypt 失败: %v", err)
	}
	
	if string(decrypted) != plaintext {
		t.Errorf("解密结果不匹配: got %s, want %s", string(decrypted), plaintext)
	}
}

func TestAESWithDifferentKeySizes(t *testing.T) {
	keySizes := []int{16, 24, 32} // AES-128, AES-192, AES-256
	plaintext := []byte("Test message for different key sizes")
	
	for _, keySize := range keySizes {
		key := make([]byte, keySize)
		for i := range key {
			key[i] = byte(i)
		}
		
		ciphertext, err := AESEncrypt(plaintext, key)
		if err != nil {
			t.Errorf("AES-%d 加密失败: %v", keySize*8, err)
			continue
		}
		
		decrypted, err := AESDecrypt(ciphertext, key)
		if err != nil {
			t.Errorf("AES-%d 解密失败: %v", keySize*8, err)
			continue
		}
		
		if string(decrypted) != string(plaintext) {
			t.Errorf("AES-%d 解密结果不匹配", keySize*8)
		}
	}
}

func TestAESInvalidKey(t *testing.T) {
	invalidKey := []byte("short") // 太短
	plaintext := []byte("test")
	
	_, err := AESEncrypt(plaintext, invalidKey)
	if err == nil {
		t.Error("使用无效密钥应返回错误")
	}
}

func TestHMACSHA256(t *testing.T) {
	keyStr := "secret-key"
	message := "Hello, HMAC!"
	
	mac := HMACSHA256(message, keyStr)
	
	// HMAC-SHA256 结果应该是 64 个十六进制字符
	if len(mac) != 64 {
		t.Errorf("HMAC-SHA256 结果长度应为 64, 实际为 %d", len(mac))
	}
	
	// 相同输入应产生相同输出
	mac2 := HMACSHA256(message, keyStr)
	if mac != mac2 {
		t.Error("相同输入应产生相同的 HMAC")
	}
	
	// 不同密钥应产生不同输出
	mac3 := HMACSHA256(message, "different-key")
	if mac == mac3 {
		t.Error("不同密钥应产生不同的 HMAC")
	}
}

func TestSecureCompare(t *testing.T) {
	a := "password123"
	b := "password123"
	c := "password456"
	
	if !SecureCompare(a, b) {
		t.Error("相同字符串应返回 true")
	}
	
	if SecureCompare(a, c) {
		t.Error("不同字符串应返回 false")
	}
	
	if SecureCompare("short", "longer") {
		t.Error("不同长度字符串应返回 false")
	}
}

func TestMaskString(t *testing.T) {
	tests := []struct {
		input        string
		visibleStart int
		visibleEnd   int
		expected     string
	}{
		{"1234567890", 2, 2, "12******90"},
		{"password", 2, 2, "pa****rd"},
		{"abc", 1, 1, "a*c"}, // len(3) > 1+1=2, 所以会遮掩中间的字符
		{"ab", 1, 1, "ab"},   // len(2) <= 1+1=2, 太短，不遮掩
		{"secret", 0, 0, "******"},
	}

	for _, tt := range tests {
		result := MaskString(tt.input, tt.visibleStart, tt.visibleEnd)
		if result != tt.expected {
			t.Errorf("MaskString(%q, %d, %d) = %s, 期望 %s", 
				tt.input, tt.visibleStart, tt.visibleEnd, result, tt.expected)
		}
	}
}

func TestDeriveKey(t *testing.T) {
	password := "myPassword"
	salt := "randomSalt"
	
	key16 := DeriveKey(password, salt, 16)
	if len(key16) != 16 {
		t.Errorf("DeriveKey 应返回 16 字节, 实际返回 %d 字节", len(key16))
	}
	
	key32 := DeriveKey(password, salt, 32)
	if len(key32) != 32 {
		t.Errorf("DeriveKey 应返回 32 字节, 实际返回 %d 字节", len(key32))
	}
	
	// 相同输入应产生相同输出
	key16_2 := DeriveKey(password, salt, 16)
	for i := range key16 {
		if key16[i] != key16_2[i] {
			t.Error("相同输入应产生相同的派生密钥")
			break
		}
	}
	
	// 不同 salt 应产生不同输出
	key16_3 := DeriveKey(password, "differentSalt", 16)
	same := true
	for i := range key16 {
		if key16[i] != key16_3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("不同 salt 应产生不同的派生密钥")
	}
}

// 基准测试
func BenchmarkMD5(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MD5("benchmark test string")
	}
}

func BenchmarkSHA256(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SHA256("benchmark test string")
	}
}

func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HashPassword("testPassword123")
	}
}

func BenchmarkAESEncrypt(b *testing.B) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("benchmark test string for AES encryption")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AESEncrypt(plaintext, key)
	}
}
