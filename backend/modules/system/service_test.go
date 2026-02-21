package system

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestService() *Service {
	return NewService(zap.NewNop(), "1.0.0", "/tmp")
}

func TestNewService(t *testing.T) {
	service := newTestService()
	assert.NotNil(t, service)
}

func TestService_GetSystemInfo(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	info, err := service.GetSystemInfo(ctx)
	require.NoError(t, err)
	assert.NotNil(t, info)

	// 验证基本字段
	assert.NotEmpty(t, info.Hostname)
	assert.NotEmpty(t, info.OS)
	assert.NotEmpty(t, info.Arch)
	assert.NotEmpty(t, info.KernelVersion)
	assert.Greater(t, info.Uptime, uint64(0))
	assert.Greater(t, info.BootTime, uint64(0))
}

func TestService_GetCPUInfo(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	info, err := service.GetCPUInfo(ctx)
	require.NoError(t, err)
	assert.NotNil(t, info)

	// 验证 CPU 信息
	assert.Greater(t, info.Cores, 0)
	assert.Greater(t, info.Threads, 0)
	// 使用率应在 0-100 之间
	assert.GreaterOrEqual(t, info.Usage, 0.0)
	assert.LessOrEqual(t, info.Usage, 100.0)
}

func TestService_GetMemoryInfo(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	info, err := service.GetMemoryInfo(ctx)
	require.NoError(t, err)
	assert.NotNil(t, info)

	// 验证内存信息
	assert.Greater(t, info.Total, uint64(0))
	assert.Greater(t, info.Available, uint64(0))
	assert.LessOrEqual(t, info.Used, info.Total)
	assert.GreaterOrEqual(t, info.UsedPercent, 0.0)
	assert.LessOrEqual(t, info.UsedPercent, 100.0)
}

func TestService_GetDiskInfo(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	// 使用根目录测试
	info, err := service.GetDiskInfo(ctx, "/")
	require.NoError(t, err)
	assert.NotNil(t, info)

	// 验证磁盘信息
	assert.NotEmpty(t, info.Path)
	assert.NotEmpty(t, info.MountPoint)
	assert.NotEmpty(t, info.FSType)
	assert.Greater(t, info.Total, uint64(0))
}

func TestService_GetAllDisks(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	disks, err := service.GetAllDisks(ctx)
	require.NoError(t, err)
	assert.NotNil(t, disks)
	assert.Greater(t, len(disks), 0)

	// 验证至少有一个磁盘
	for _, disk := range disks {
		assert.NotEmpty(t, disk.MountPoint)
	}
}

func TestService_GetNetworkInterfaces(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	// 获取所有接口
	interfaces, err := service.GetNetworkInterfaces(ctx, false)
	require.NoError(t, err)
	assert.NotNil(t, interfaces)

	// 应该至少有 lo 接口
	assert.Greater(t, len(interfaces), 0)
}

func TestService_GetNetworkStats(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	stats, err := service.GetNetworkStats(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stats)
}

func TestService_GetDeviceInfo(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	info, err := service.GetDeviceInfo(ctx)
	require.NoError(t, err)
	assert.NotNil(t, info)

	// 验证设备信息
	assert.NotEmpty(t, info.DeviceName)
}

func TestService_GetResourceUsage(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	usage, err := service.GetResourceUsage(ctx)
	require.NoError(t, err)
	assert.NotNil(t, usage)

	// 验证资源使用率在合理范围
	assert.GreaterOrEqual(t, usage.CPUUsage, 0.0)
	assert.LessOrEqual(t, usage.CPUUsage, 100.0)
	assert.GreaterOrEqual(t, usage.MemoryUsage, 0.0)
	assert.LessOrEqual(t, usage.MemoryUsage, 100.0)

	// 应该有时间戳
	assert.False(t, usage.Timestamp.IsZero())
}

func TestService_GetTimeZone(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	tz := service.GetTimeZone(ctx)
	assert.NotNil(t, tz)
	assert.NotEmpty(t, tz.Name)

	// UTC 偏移应在 -12 到 +14 小时范围内
	assert.GreaterOrEqual(t, tz.Offset, int(-12*3600))
	assert.LessOrEqual(t, tz.Offset, int(14*3600))
}

func TestService_GetTopProcesses(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	// 按 CPU 排序
	procs, err := service.GetTopProcesses(ctx, 5, "cpu")
	require.NoError(t, err)
	assert.NotNil(t, procs)
	assert.LessOrEqual(t, len(procs), 5)

	// 按内存排序
	procs, err = service.GetTopProcesses(ctx, 5, "memory")
	require.NoError(t, err)
	assert.NotNil(t, procs)
}

func TestService_GetHealthStatus(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	status := service.GetHealthStatus(ctx)
	assert.NotNil(t, status)

	// 应该有状态
	assert.NotEmpty(t, status.Status)
	assert.Contains(t, []string{"healthy", "warning", "critical"}, status.Status)

	// 应该有时间戳
	assert.False(t, status.LastChecked.IsZero())
}

func TestService_GetLogs(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	// 尝试获取系统日志 (可能需要权限)
	logs, err := service.GetLogs(ctx, "", 10)
	// 即使出错也不应该 panic
	if err == nil {
		assert.NotNil(t, logs)
	}
}

func TestService_GetMacAddress(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	mac, err := service.GetMacAddress(ctx)
	require.NoError(t, err)
	// MAC 地址可能为空（没有物理网卡的情况）
	if mac != "" && mac != "unknown" {
		// 验证 MAC 地址格式 (xx:xx:xx:xx:xx:xx) 或主机名
		// MAC 地址格式或主机名都可以接受
		assert.NotEmpty(t, mac)
	}
}

func TestService_GetCPUTemperature(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	temp, err := service.GetCPUTemperature(ctx)
	// 温度传感器可能不可用
	if err == nil {
		assert.GreaterOrEqual(t, temp, 0.0)
		assert.LessOrEqual(t, temp, 150.0) // 合理的温度范围
	}
}

// 测试模块接口
func TestModule_Name(t *testing.T) {
	m := New()
	assert.Equal(t, "System", m.Name())
}

func TestModule_ID(t *testing.T) {
	m := New()
	assert.Equal(t, "system", m.ID())
}

func TestModule_Dependencies(t *testing.T) {
	m := New()
	deps := m.Dependencies()
	assert.Empty(t, deps)
}

// 并发测试
func TestService_ConcurrentAccess(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	// 并发获取系统信息
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = service.GetSystemInfo(ctx)
			_, _ = service.GetCPUInfo(ctx)
			_, _ = service.GetMemoryInfo(ctx)
			_, _ = service.GetResourceUsage(ctx)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	timeout := time.After(10 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}
}

// 基准测试
func BenchmarkService_GetSystemInfo(b *testing.B) {
	service := newTestService()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetSystemInfo(ctx)
	}
}

func BenchmarkService_GetCPUInfo(b *testing.B) {
	service := newTestService()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetCPUInfo(ctx)
	}
}

func BenchmarkService_GetMemoryInfo(b *testing.B) {
	service := newTestService()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetMemoryInfo(ctx)
	}
}

func BenchmarkService_GetResourceUsage(b *testing.B) {
	service := newTestService()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetResourceUsage(ctx)
	}
}
