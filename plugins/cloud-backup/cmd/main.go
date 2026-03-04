// rde-cloud-backup 云备份模块独立插件
// 包含云端备份、恢复等功能
package main

import (
	"fmt"
	"os"

	cloud_backup "github.com/ruizi-store/rde-plugin-cloud-backup"
	"github.com/ruizi-store/rde-plugin-common/go/sdk"
)

func main() {
	p := sdk.New("rde-cloud-backup", "1.0.0")

	// 启用 SQLite（云备份自身需要数据库）
	p.EnableDB()

	// 注册模块
	p.AddModule(cloud_backup.New())

	if err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
