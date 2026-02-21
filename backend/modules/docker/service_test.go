// Package docker Docker 模块测试
package docker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestService(t *testing.T) *Service {
	t.Helper()
	logger := zap.NewNop()
	// 创建服务（可能没有 Docker daemon）
	svc, err := NewService(logger)
	assert.NoError(t, err) // NewService 不应返回错误，即使 Docker 不可用
	return svc
}

// ----- 服务创建测试 -----

func TestNewService(t *testing.T) {
	logger := zap.NewNop()
	svc, err := NewService(logger)

	// 即使 Docker 未安装也应成功创建服务
	assert.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestService_Close(t *testing.T) {
	svc := setupTestService(t)

	// 关闭服务不应出错
	err := svc.Close()
	assert.NoError(t, err)
}

// ----- Docker 状态测试 -----

func TestService_IsRunning(t *testing.T) {
	svc := setupTestService(t)
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试 IsRunning 不会崩溃
	// 结果取决于环境中是否有 Docker
	running := svc.IsRunning(ctx)
	t.Logf("Docker running: %v", running)
}

func TestService_GetInfo_NoDocker(t *testing.T) {
	logger := zap.NewNop()
	// 创建一个没有 client 的服务
	svc := &Service{logger: logger, client: nil}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := svc.GetInfo(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker client not available")
}

func TestService_ListContainers_NoDocker(t *testing.T) {
	logger := zap.NewNop()
	svc := &Service{logger: logger, client: nil}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := svc.ListContainers(ctx, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker client not available")
}

func TestService_ListImages_NoDocker(t *testing.T) {
	logger := zap.NewNop()
	svc := &Service{logger: logger, client: nil}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := svc.ListImages(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker client not available")
}

func TestService_ListNetworks_NoDocker(t *testing.T) {
	logger := zap.NewNop()
	svc := &Service{logger: logger, client: nil}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := svc.ListNetworks(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker client not available")
}

func TestService_PullImage_NoDocker(t *testing.T) {
	logger := zap.NewNop()
	svc := &Service{logger: logger, client: nil}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := svc.PullImage(ctx, "alpine:latest", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker client not available")
}

// ----- 类型测试 -----

func TestContainerConfig_Defaults(t *testing.T) {
	config := ContainerConfig{
		Name:  "test",
		Image: "nginx:latest",
	}

	assert.Equal(t, "test", config.Name)
	assert.Equal(t, "nginx:latest", config.Image)
	assert.Nil(t, config.Ports)
	assert.Nil(t, config.Volumes)
	assert.Nil(t, config.Environment)
	assert.False(t, config.Privileged)
}

func TestCreateContainerRequest_Structure(t *testing.T) {
	req := CreateContainerRequest{
		Name:  "my-container",
		Image: "redis:7",
		Ports: map[string]string{
			"6379/tcp": "6379",
		},
		Volumes: map[string]string{
			"/data/redis": "/data",
		},
		Environment: []string{
			"REDIS_PASSWORD=secret",
		},
		Restart: "unless-stopped",
	}

	assert.Equal(t, "my-container", req.Name)
	assert.Equal(t, "redis:7", req.Image)
	assert.Len(t, req.Ports, 1)
	assert.Equal(t, "6379", req.Ports["6379/tcp"])
	assert.Len(t, req.Volumes, 1)
	assert.Len(t, req.Environment, 1)
}

func TestDockerInfo_Structure(t *testing.T) {
	info := DockerInfo{
		Version:           "24.0.0",
		APIVersion:        "1.43",
		OS:                "linux",
		Arch:              "amd64",
		Containers:        10,
		ContainersRunning: 5,
		ContainersStopped: 5,
		Images:            20,
		MemTotal:          16000000000,
		NCPU:              8,
	}

	assert.Equal(t, "24.0.0", info.Version)
	assert.Equal(t, "linux", info.OS)
	assert.Equal(t, 10, info.Containers)
	assert.Equal(t, 5, info.ContainersRunning)
	assert.Equal(t, 8, info.NCPU)
}

func TestContainerStats_Structure(t *testing.T) {
	stats := ContainerStats{
		CPUPercent:    25.5,
		MemoryUsage:   1073741824,
		MemoryLimit:   8589934592,
		MemoryPercent: 12.5,
		NetworkRx:     1000000,
		NetworkTx:     500000,
	}

	assert.Equal(t, 25.5, stats.CPUPercent)
	assert.Equal(t, uint64(1073741824), stats.MemoryUsage)
	assert.Equal(t, 12.5, stats.MemoryPercent)
}

func TestPortBinding_Structure(t *testing.T) {
	binding := PortBinding{
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	assert.Equal(t, "8080", binding.HostPort)
	assert.Equal(t, "80", binding.ContainerPort)
	assert.Equal(t, "tcp", binding.Protocol)
}

func TestNetworkInfo_Structure(t *testing.T) {
	network := NetworkInfo{
		ID:      "abc123",
		Name:    "my-network",
		Driver:  "bridge",
		Scope:   "local",
		Subnet:  "172.18.0.0/16",
		Gateway: "172.18.0.1",
	}

	assert.Equal(t, "abc123", network.ID)
	assert.Equal(t, "my-network", network.Name)
	assert.Equal(t, "bridge", network.Driver)
}

func TestImageInfo_Structure(t *testing.T) {
	image := ImageInfo{
		ID:         "sha256:abc123",
		Repository: "nginx",
		Tag:        "latest",
		Size:       142000000,
		Created:    1700000000,
	}

	assert.Equal(t, "sha256:abc123", image.ID)
	assert.Equal(t, "nginx", image.Repository)
	assert.Equal(t, "latest", image.Tag)
}
