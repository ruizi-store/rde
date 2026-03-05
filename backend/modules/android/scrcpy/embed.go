// Package scrcpy - scrcpy-server.jar 嵌入
package scrcpy

import (
	_ "embed"
)

// scrcpy-server.jar 会被嵌入到二进制中
// 需要从 https://github.com/Genymobile/scrcpy/releases 下载 v3.3.4 版本
//
//go:embed assets/scrcpy-server.jar
var serverJar []byte

// GetServerJar 获取嵌入的 scrcpy-server.jar
func GetServerJar() []byte {
	return serverJar
}

// ServerJarSize 获取 jar 文件大小
func ServerJarSize() int {
	return len(serverJar)
}
