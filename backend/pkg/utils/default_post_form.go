package utils

import "github.com/labstack/echo/v4"

// DefaultPostForm 获取 POST 表单参数，如果不存在则返回默认值
func DefaultPostForm(c echo.Context, key, defaultValue string) string {
	value := c.FormValue(key)
	if value == "" {
		return defaultValue
	}
	return value
}
