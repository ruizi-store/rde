package random

import (
	"regexp"
	"strings"
	"testing"
)

func TestString(t *testing.T) {
	lengths := []int{0, 1, 8, 16, 32, 64}
	
	for _, length := range lengths {
		t.Run("length_"+string(rune('0'+length)), func(t *testing.T) {
			s := String(length)
			if len(s) != length {
				t.Errorf("String(%d) 生成的字符串长度为 %d, 期望 %d", length, len(s), length)
			}
		})
	}
}

func TestStringUniqueness(t *testing.T) {
	// 生成多个随机字符串，检查是否有重复
	generated := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		s := String(16)
		if generated[s] {
			t.Errorf("生成了重复的随机字符串: %s", s)
		}
		generated[s] = true
	}
}

func TestStringWithCharset(t *testing.T) {
	tests := []struct {
		name    string
		length  int
		charset string
		pattern string
	}{
		{"numeric", 10, CharsetNumeric, "^[0-9]+$"},
		{"alpha_lower", 10, CharsetAlphaLower, "^[a-z]+$"},
		{"alpha_upper", 10, CharsetAlphaUpper, "^[A-Z]+$"},
		{"hex", 10, CharsetHex, "^[0-9a-f]+$"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := StringWithCharset(tt.length, tt.charset)
			if len(s) != tt.length {
				t.Errorf("长度不正确: got %d, want %d", len(s), tt.length)
			}
			
			matched, _ := regexp.MatchString(tt.pattern, s)
			if !matched {
				t.Errorf("字符串 %s 不匹配模式 %s", s, tt.pattern)
			}
		})
	}
}

func TestNumeric(t *testing.T) {
	s := Numeric(10)
	if len(s) != 10 {
		t.Errorf("Numeric(10) 长度应为 10, 实际为 %d", len(s))
	}
	
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Errorf("Numeric 应只包含数字, 但包含了 %c", c)
		}
	}
}

func TestAlpha(t *testing.T) {
	s := Alpha(10)
	if len(s) != 10 {
		t.Errorf("Alpha(10) 长度应为 10, 实际为 %d", len(s))
	}
	
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			t.Errorf("Alpha 应只包含字母, 但包含了 %c", c)
		}
	}
}

func TestUUID(t *testing.T) {
	u := UUID()
	
	// UUID 格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	matched, _ := regexp.MatchString(pattern, u)
	if !matched {
		t.Errorf("UUID 格式不正确: %s", u)
	}
}

func TestUUIDShort(t *testing.T) {
	u := UUIDShort()
	
	// UUIDShort 长度应为 32 (不含横线)
	if len(u) != 32 {
		t.Errorf("UUIDShort 长度应为 32, 实际为 %d", len(u))
	}
	
	// 应只包含十六进制字符
	for _, c := range u {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("UUIDShort 应只包含十六进制字符, 但包含了 %c", c)
		}
	}
}

func TestInt(t *testing.T) {
	for i := 0; i < 1000; i++ {
		n := Int(10, 20)
		if n < 10 || n > 20 {
			t.Errorf("Int(10, 20) = %d, 应在 [10, 20] 范围内", n)
		}
	}
}

func TestIntEdgeCases(t *testing.T) {
	// min == max
	n := Int(5, 5)
	if n != 5 {
		t.Errorf("Int(5, 5) = %d, 应返回 5", n)
	}
	
	// min > max
	n = Int(10, 5)
	if n != 10 {
		t.Errorf("Int(10, 5) = %d, 应返回 min (10)", n)
	}
}

func TestInt64(t *testing.T) {
	for i := 0; i < 1000; i++ {
		n := Int64(100, 200)
		if n < 100 || n > 200 {
			t.Errorf("Int64(100, 200) = %d, 应在 [100, 200] 范围内", n)
		}
	}
}

func TestFloat64(t *testing.T) {
	for i := 0; i < 1000; i++ {
		f := Float64()
		if f < 0.0 || f >= 1.0 {
			t.Errorf("Float64() = %f, 应在 [0.0, 1.0) 范围内", f)
		}
	}
}

func TestFloat64Range(t *testing.T) {
	for i := 0; i < 1000; i++ {
		f := Float64Range(10.0, 20.0)
		if f < 10.0 || f >= 20.0 {
			t.Errorf("Float64Range(10.0, 20.0) = %f, 应在 [10.0, 20.0) 范围内", f)
		}
	}
}

func TestBool(t *testing.T) {
	trueCount := 0
	falseCount := 0
	iterations := 10000
	
	for i := 0; i < iterations; i++ {
		if Bool() {
			trueCount++
		} else {
			falseCount++
		}
	}
	
	// 检查分布是否大致均匀 (允许 10% 误差)
	expectedEach := iterations / 2
	tolerance := iterations / 10
	
	if trueCount < expectedEach-tolerance || trueCount > expectedEach+tolerance {
		t.Errorf("Bool() 分布不均匀: true=%d, false=%d", trueCount, falseCount)
	}
}

func TestBytes(t *testing.T) {
	lengths := []int{0, 1, 16, 32, 64}
	
	for _, length := range lengths {
		b := Bytes(length)
		if len(b) != length {
			t.Errorf("Bytes(%d) 长度为 %d, 期望 %d", length, len(b), length)
		}
	}
}

func TestSecureString(t *testing.T) {
	s, err := SecureString(32)
	if err != nil {
		t.Errorf("SecureString 返回错误: %v", err)
	}
	
	if len(s) != 32 {
		t.Errorf("SecureString(32) 长度为 %d, 期望 32", len(s))
	}
}

func TestSecureBytes(t *testing.T) {
	b, err := SecureBytes(32)
	if err != nil {
		t.Errorf("SecureBytes 返回错误: %v", err)
	}
	
	if len(b) != 32 {
		t.Errorf("SecureBytes(32) 长度为 %d, 期望 32", len(b))
	}
}

func TestShuffle(t *testing.T) {
	original := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	slice := make([]int, len(original))
	copy(slice, original)
	
	Shuffle(slice)
	
	// 检查元素是否都还在
	sum := 0
	for _, v := range slice {
		sum += v
	}
	if sum != 55 {
		t.Error("Shuffle 改变了元素")
	}
	
	// 多次打乱应该产生不同顺序（概率极小会相同）
	sameCount := 0
	for i := 0; i < 100; i++ {
		slice2 := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		Shuffle(slice2)
		
		same := true
		for j := range slice2 {
			if slice2[j] != original[j] {
				same = false
				break
			}
		}
		if same {
			sameCount++
		}
	}
	
	// 100 次中超过 10 次完全相同是不正常的
	if sameCount > 10 {
		t.Error("Shuffle 似乎没有正确打乱顺序")
	}
}

func TestPick(t *testing.T) {
	items := []string{"apple", "banana", "cherry", "date"}
	
	for i := 0; i < 100; i++ {
		item := Pick(items)
		found := false
		for _, v := range items {
			if v == item {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Pick 返回了不存在的元素: %s", item)
		}
	}
}

func TestPickN(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	
	// 取 5 个样本
	sample := PickN(items, 5)
	if len(sample) != 5 {
		t.Errorf("PickN(items, 5) 长度为 %d, 期望 5", len(sample))
	}
	
	// 检查无重复
	seen := make(map[int]bool)
	for _, v := range sample {
		if seen[v] {
			t.Errorf("PickN 返回了重复元素: %d", v)
		}
		seen[v] = true
	}
	
	// 取超过长度的样本
	sample = PickN(items, 20)
	if len(sample) != len(items) {
		t.Errorf("PickN 超长度时应返回全部元素")
	}
}

// 基准测试
func BenchmarkString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		String(32)
	}
}

func BenchmarkSecureString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SecureString(32)
	}
}

func BenchmarkUUID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		UUID()
	}
}
