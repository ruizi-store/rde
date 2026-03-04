// rde-ai AI 模块独立插件
// 包含 AI 聊天、语音、Webhook 等功能
package main

import (
	"fmt"
	"io/fs"
	"os"

	ai "github.com/ruizi-store/rde-plugin-ai"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
)

func main() {
	p := sdk.New("rde-ai", "1.0.0")

	// 启用 SQLite
	p.EnableDB()

	// 注册模块
	p.AddModule(ai.New())

	// 注册前端（从嵌入的 embed.FS 提供）
	frontendFS, _ := fs.Sub(ai.FrontendDist, "frontend/dist")
	p.ServeFrontendFS("/app/ai", frontendFS)

	if err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
