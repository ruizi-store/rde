package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// ==================== Hash 函数 ====================

// MD5 计算字符串的 MD5 哈希
func MD5(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// MD5Bytes 计算字节数组的 MD5 哈希
func MD5Bytes(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// MD5File 计算文件的 MD5 哈希
func MD5File(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SHA256 计算字符串的 SHA256 哈希
func SHA256(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

// SHA256Bytes 计算字节数组的 SHA256 哈希
func SHA256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// SHA256File 计算文件的 SHA256 哈希
func SHA256File(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SHA512 计算字符串的 SHA512 哈希
func SHA512(text string) string {
	hash := sha512.Sum512([]byte(text))
	return hex.EncodeToString(hash[:])
}

// ==================== HMAC 函数 ====================

// HMACSHA256 使用 HMAC-SHA256 签名
func HMACSHA256(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// HMACSHA512 使用 HMAC-SHA512 签名
func HMACSHA512(message, secret string) string {
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC 验证 HMAC 签名
func VerifyHMAC(message, secret, signature string) bool {
	expected := HMACSHA256(message, secret)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ==================== 密码哈希 ====================

// HashPassword 使用 bcrypt 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// HashPasswordWithCost 使用指定强度哈希密码
func HashPasswordWithCost(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ==================== AES 加密 ====================

// AESEncrypt 使用 AES-GCM 加密
func AESEncrypt(plaintext, key []byte) ([]byte, error) {
	// 密钥长度必须是 16, 24, 或 32 字节
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid key size: must be 16, 24, or 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// AESDecrypt 使用 AES-GCM 解密
func AESDecrypt(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, errors.New("invalid key size: must be 16, 24, or 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// AESEncryptString 加密字符串并返回 Base64 编码
func AESEncryptString(plaintext, key string) (string, error) {
	keyBytes := []byte(key)
	// 如果密钥长度不足，使用 SHA256 生成 32 字节密钥
	if len(keyBytes) < 32 {
		hash := sha256.Sum256(keyBytes)
		keyBytes = hash[:]
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	encrypted, err := AESEncrypt([]byte(plaintext), keyBytes)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// AESDecryptString 解密 Base64 编码的密文
func AESDecryptString(ciphertext, key string) (string, error) {
	keyBytes := []byte(key)
	if len(keyBytes) < 32 {
		hash := sha256.Sum256(keyBytes)
		keyBytes = hash[:]
	} else if len(keyBytes) > 32 {
		keyBytes = keyBytes[:32]
	}

	encrypted, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	decrypted, err := AESDecrypt(encrypted, keyBytes)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// ==================== Base64 编码 ====================

// Base64Encode Base64 编码
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode Base64 解码
func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// Base64URLEncode URL 安全的 Base64 编码
func Base64URLEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

// Base64URLDecode URL 安全的 Base64 解码
func Base64URLDecode(encoded string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(encoded)
}

// ==================== 随机数生成 ====================

// GenerateKey 生成指定长度的随机密钥
func GenerateKey(length int) ([]byte, error) {
	key := make([]byte, length)
	_, err := rand.Read(key)
	return key, err
}

// GenerateHexKey 生成十六进制格式的随机密钥
func GenerateHexKey(length int) (string, error) {
	key, err := GenerateKey(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// GenerateSalt 生成盐值
func GenerateSalt(length int) (string, error) {
	return GenerateHexKey(length)
}

// ==================== 工具函数 ====================

// XOR 对两个字节数组进行异或操作
func XOR(a, b []byte) []byte {
	if len(a) != len(b) {
		return nil
	}
	result := make([]byte, len(a))
	for i := range a {
		result[i] = a[i] ^ b[i]
	}
	return result
}

// PadPKCS7 PKCS7 填充
func PadPKCS7(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// UnpadPKCS7 移除 PKCS7 填充
func UnpadPKCS7(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) {
		return nil, errors.New("invalid padding")
	}
	return data[:len(data)-padding], nil
}

// SecureCompare 安全比较，防止时序攻击
func SecureCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal([]byte(a), []byte(b))
}

// MaskString 遮掩字符串（如密码、密钥）
func MaskString(s string, visibleStart, visibleEnd int) string {
	if len(s) <= visibleStart+visibleEnd {
		return s
	}
	masked := s[:visibleStart]
	for i := 0; i < len(s)-visibleStart-visibleEnd; i++ {
		masked += "*"
	}
	masked += s[len(s)-visibleEnd:]
	return masked
}

// DeriveKey 从密码派生密钥（简单版本）
func DeriveKey(password, salt string, keyLen int) []byte {
	combined := fmt.Sprintf("%s:%s", password, salt)
	hash := sha256.Sum256([]byte(combined))
	if keyLen <= 32 {
		return hash[:keyLen]
	}
	// 对于更长的密钥，重复哈希
	result := hash[:]
	for len(result) < keyLen {
		hash = sha256.Sum256(append(hash[:], []byte(combined)...))
		result = append(result, hash[:]...)
	}
	return result[:keyLen]
}

// ==================== CasaOS 兼容别名 ====================

// GetMD5ByStr 获取字符串的 MD5 哈希 - CasaOS 兼容别名
func GetMD5ByStr(str string) string {
	return MD5(str)
}

// GetMD5FromFile 获取文件的 MD5 哈希 - CasaOS 兼容别名
func GetMD5FromFile(filePath string) (string, error) {
	return MD5File(filePath)
}
