package random

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	mrand "math/rand"
	"time"

	"github.com/google/uuid"
)

const (
	// 字符集
	CharsetAlphaNum   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	CharsetAlpha      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsetAlphaLower = "abcdefghijklmnopqrstuvwxyz"
	CharsetAlphaUpper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsetNumeric    = "0123456789"
	CharsetHex        = "0123456789abcdef"
	CharsetSpecial    = "!@#$%^&*()_+-=[]{}|;':\",./<>?"
)

func init() {
	mrand.Seed(time.Now().UnixNano())
}

// String 生成指定长度的随机字符串（字母+数字）
func String(length int) string {
	return StringWithCharset(length, CharsetAlphaNum)
}

// StringWithCharset 使用指定字符集生成随机字符串
func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[mrand.Intn(len(charset))]
	}
	return string(b)
}

// SecureString 使用加密安全随机数生成字符串
func SecureString(length int) (string, error) {
	return SecureStringWithCharset(length, CharsetAlphaNum)
}

// SecureStringWithCharset 使用加密安全随机数生成指定字符集的字符串
func SecureStringWithCharset(length int, charset string) (string, error) {
	b := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))
	
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// Numeric 生成随机数字字符串
func Numeric(length int) string {
	return StringWithCharset(length, CharsetNumeric)
}

// Alpha 生成随机字母字符串
func Alpha(length int) string {
	return StringWithCharset(length, CharsetAlpha)
}

// AlphaLower 生成随机小写字母字符串
func AlphaLower(length int) string {
	return StringWithCharset(length, CharsetAlphaLower)
}

// AlphaUpper 生成随机大写字母字符串
func AlphaUpper(length int) string {
	return StringWithCharset(length, CharsetAlphaUpper)
}

// Hex 生成随机十六进制字符串
func Hex(length int) string {
	return StringWithCharset(length, CharsetHex)
}

// UUID 生成 UUID v4
func UUID() string {
	return uuid.New().String()
}

// UUIDShort 生成不带横线的 UUID
func UUIDShort() string {
	u := uuid.New()
	return hex.EncodeToString(u[:])
}

// Int 生成指定范围内的随机整数 [min, max]
func Int(min, max int) int {
	if min >= max {
		return min
	}
	return mrand.Intn(max-min+1) + min
}

// Int64 生成指定范围内的随机 int64 [min, max]
func Int64(min, max int64) int64 {
	if min >= max {
		return min
	}
	return mrand.Int63n(max-min+1) + min
}

// Float64 生成 [0.0, 1.0) 范围内的随机浮点数
func Float64() float64 {
	return mrand.Float64()
}

// Float64Range 生成指定范围内的随机浮点数 [min, max)
func Float64Range(min, max float64) float64 {
	return min + mrand.Float64()*(max-min)
}

// Bool 生成随机布尔值
func Bool() bool {
	return mrand.Intn(2) == 1
}

// Bytes 生成随机字节数组
func Bytes(length int) []byte {
	b := make([]byte, length)
	mrand.Read(b)
	return b
}

// SecureBytes 使用加密安全随机数生成字节数组
func SecureBytes(length int) ([]byte, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Shuffle 随机打乱切片顺序
func Shuffle[T any](slice []T) {
	mrand.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

// Pick 从切片中随机选择一个元素
func Pick[T any](slice []T) T {
	var zero T
	if len(slice) == 0 {
		return zero
	}
	return slice[mrand.Intn(len(slice))]
}

// PickN 从切片中随机选择 n 个元素（不重复）
func PickN[T any](slice []T, n int) []T {
	if n >= len(slice) {
		result := make([]T, len(slice))
		copy(result, slice)
		Shuffle(result)
		return result
	}

	// 复制切片以避免修改原始数据
	temp := make([]T, len(slice))
	copy(temp, slice)
	Shuffle(temp)
	return temp[:n]
}

// WeightedPick 根据权重随机选择
func WeightedPick(weights []int) int {
	if len(weights) == 0 {
		return -1
	}

	total := 0
	for _, w := range weights {
		total += w
	}

	if total == 0 {
		return mrand.Intn(len(weights))
	}

	r := mrand.Intn(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return i
		}
	}
	return len(weights) - 1
}

// Token 生成用于认证的安全令牌
func Token() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Password 生成随机密码（包含大小写字母、数字和特殊字符）
func Password(length int) string {
	if length < 4 {
		length = 4
	}

	// 确保包含各类字符
	password := make([]byte, length)
	password[0] = CharsetAlphaLower[mrand.Intn(len(CharsetAlphaLower))]
	password[1] = CharsetAlphaUpper[mrand.Intn(len(CharsetAlphaUpper))]
	password[2] = CharsetNumeric[mrand.Intn(len(CharsetNumeric))]
	password[3] = CharsetSpecial[mrand.Intn(len(CharsetSpecial))]

	// 填充剩余字符
	allChars := CharsetAlphaNum + CharsetSpecial
	for i := 4; i < length; i++ {
		password[i] = allChars[mrand.Intn(len(allChars))]
	}

	// 打乱顺序
	for i := len(password) - 1; i > 0; i-- {
		j := mrand.Intn(i + 1)
		password[i], password[j] = password[j], password[i]
	}

	return string(password)
}
