// Package android 前端静态资源嵌入
// 构建时通过 Makefile 将 frontend/dist 复制到 backend/frontend_dist/
package android

import "embed"

//go:embed frontend_dist/*
var FrontendDist embed.FS
