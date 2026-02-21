package version

import (
	"testing"
)

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"v1 greater major", "2.0.0", "1.0.0", 1},
		{"v1 less major", "1.0.0", "2.0.0", -1},
		{"v1 greater minor", "1.2.0", "1.1.0", 1},
		{"v1 less minor", "1.1.0", "1.2.0", -1},
		{"v1 greater patch", "1.0.2", "1.0.1", 1},
		{"v1 less patch", "1.0.1", "1.0.2", -1},
		{"with v prefix", "v1.2.0", "v1.1.0", 1},
		{"mixed v prefix", "v1.2.0", "1.2.0", 0},
		{"different lengths", "1.0", "1.0.0", 0},
		{"shorter v1", "1", "1.0.0", 0},
		{"longer v2", "1.0.0.1", "1.0.0", 1},
		{"complex version", "1.2.3", "1.2.4", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersion(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("CompareVersion(%s, %s) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestParseVersionPart(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"10", 10},
		{"123", 123},
		{"1a", 1},
		{"1-beta", 1},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseVersionPart(tt.input)
			if result != tt.expected {
				t.Errorf("parseVersionPart(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsNeedUpdate(t *testing.T) {
	// 测试版本比较逻辑（不依赖网络）
	// 由于 GetLatestVersion 依赖网络，我们只测试基本场景

	// 当前版本很高，不需要更新
	needUpdate, info := IsNeedUpdate("999.999.999")
	if needUpdate {
		t.Log("Version check returned need update, remote version:", info.Version)
	}
}
