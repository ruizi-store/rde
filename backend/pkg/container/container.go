// Package container 提供运行环境（物理机/虚拟机/容器）探测。
//
// 用途：RDE 同时以 DEB（拥有整台机器）和 Docker 镜像（单容器）两种形态交付，
// 部分模块与安装向导的行为需要按环境区分（如 systemd 服务管理仅在宿主机可用）。
package container

import (
	"os"
	"strings"
	"sync"
)

var (
	detectOnce sync.Once
	inCont     bool
)

// InContainer 判断当前进程是否运行在容器内。
//
// 判定优先级：
//  1. 显式环境变量 RDE_CONTAINER=1（官方镜像中由 Dockerfile 设定，最可靠）；
//  2. /.dockerenv 存在（Docker 运行时标记）；
//  3. /proc/1/cgroup 含有 docker/containerd/kubepods 关键字。
//
// 结果进程内缓存，可安全地在热路径调用。
func InContainer() bool {
	detectOnce.Do(func() {
		inCont = detect()
	})
	return inCont
}

func detect() bool {
	if v := os.Getenv("RDE_CONTAINER"); v == "1" || strings.EqualFold(v, "true") {
		return true
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// /proc/1/cgroup 在 cgroup v1/v2 下均可读取；读取失败静默跳过
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		for _, kw := range []string{"docker", "containerd", "kubepods"} {
			if strings.Contains(s, kw) {
				return true
			}
		}
	}

	return false
}
