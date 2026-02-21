package backup

import (
	"encoding/json"
)

// parseJSONStringArray 解析 JSON 字符串数组
func parseJSONStringArray(s string) []string {
	if s == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil
	}
	return arr
}

// toJSONString 转换为 JSON 字符串
func toJSONString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
