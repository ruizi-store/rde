// rde-android Android 投屏/控制模块独立插件
// 包含 ADB 连接、Scrcpy 投屏等功能
package main

import (
	"fmt"
	"io/fs"
	"os"

	android "github.com/ruizi-store/rde-plugin-android/backend"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
)

func main() {
	p := sdk.New("rde-android", "1.0.0")

	// 注册模块
	p.AddModule(android.New())

	// 注册嵌入的前端静态文件
	frontendFS, err := fs.Sub(android.FrontendDist, "frontend_dist")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: failed to load embedded frontend: %v\n", err)
		os.Exit(1)
	}
	p.ServeFrontendFS("/app/android", frontendFS)

	if err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
