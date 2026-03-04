// rde-vm 虚拟机管理模块独立插件
// 包含 QEMU/KVM 虚拟机管理、VNC 代理等功能
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ruizi-store/rde-plugin-common/go/sdk"
	vm "github.com/ruizi-store/rde-plugin-vm"
)

func main() {
	p := sdk.New("rde-vm", "1.0.0")

	// 注册模块
	p.AddModule(vm.New())

	// 注册前端静态文件
	exe, _ := os.Executable()
	execDir := filepath.Dir(exe)
	p.ServeFrontend("/app/vm", filepath.Join(execDir, "frontend", "vm"))

	if err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
