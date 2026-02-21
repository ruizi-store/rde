package utils

import (
	"fmt"
	
	"github.com/labstack/echo/v4"
)

// DefaultQuery 获取查询参数，如果不存在则返回默认值
func DefaultQuery(c echo.Context, key, defaultValue string) string {
	value := c.QueryParam(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// DefaultQueryInt 获取整数查询参数，如果不存在则返回默认值
func DefaultQueryInt(c echo.Context, key string, defaultValue int) int {
	value := c.QueryParam(key)
	if value == "" {
		return defaultValue
	}
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	if err != nil {
		return defaultValue
	}
	return result
}
