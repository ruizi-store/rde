// Package ai 嵌入前端构建产物
package ai

import "embed"

// FrontendDist 嵌入前端构建产物 (frontend/dist)
// 构建前需要 ln -sfn ../../frontend/dist frontend/dist
//
//go:embed all:frontend/dist
var FrontendDist embed.FS
