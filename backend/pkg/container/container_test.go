package container

import (
	"os"
	"testing"
)

func TestInContainerByEnv(t *testing.T) {
	// 环境变量优先且缓存按首次调用固化，这里直接测试 detect 的判定逻辑
	t.Run("RDE_CONTAINER=1", func(t *testing.T) {
		t.Setenv("RDE_CONTAINER", "1")
		if !detect() {
			t.Error("RDE_CONTAINER=1 should be detected as container")
		}
	})

	t.Run("RDE_CONTAINER=true（大小写不敏感）", func(t *testing.T) {
		t.Setenv("RDE_CONTAINER", "TRUE")
		if !detect() {
			t.Error("RDE_CONTAINER=TRUE should be detected as container")
		}
	})

	t.Run("RDE_CONTAINER=0", func(t *testing.T) {
		t.Setenv("RDE_CONTAINER", "0")
		// 宿主机上无 /.dockerenv 时应返回 false
		if _, err := os.Stat("/.dockerenv"); err != nil && detect() {
			t.Error("RDE_CONTAINER=0 on a host should not be detected as container")
		}
	})
}
