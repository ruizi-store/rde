// Package docker Docker 服务
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

// Service Docker 服务
type Service struct {
	client *client.Client
	logger *zap.Logger
}

// NewService 创建服务
func NewService(logger *zap.Logger) (*Service, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		logger.Warn("Failed to create Docker client", zap.Error(err))
		return &Service{logger: logger}, nil
	}

	// 异步测试连接，不阻塞启动
	s := &Service{
		client: cli,
		logger: logger,
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if _, err := cli.Ping(ctx); err != nil {
			logger.Warn("Docker is not running", zap.Error(err))
		} else {
			logger.Info("Docker daemon connected")
		}
	}()

	return s, nil
}

// Close 关闭服务
func (s *Service) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// IsRunning 检查 Docker 是否运行
func (s *Service) IsRunning(ctx context.Context) bool {
	if s.client == nil {
		return false
	}
	_, err := s.client.Ping(ctx)
	return err == nil
}

// GetInfo 获取 Docker 信息
func (s *Service) GetInfo(ctx context.Context) (*DockerInfo, error) {
	if s.client == nil {
		return nil, errors.New("docker client not available")
	}

	info, err := s.client.Info(ctx)
	if err != nil {
		return nil, err
	}

	return &DockerInfo{
		Version:           info.ServerVersion,
		APIVersion:        s.client.ClientVersion(),
		OS:                info.OperatingSystem,
		Arch:              info.Architecture,
		KernelVersion:     info.KernelVersion,
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		ContainersStopped: info.ContainersStopped,
		Images:            info.Images,
		Driver:            info.Driver,
		MemTotal:          info.MemTotal,
		NCPU:              info.NCPU,
	}, nil
}

// PullImage 拉取镜像
func (s *Service) PullImage(ctx context.Context, image string, progress chan<- string) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}

	reader, err := s.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	for {
		var event map[string]interface{}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if progress != nil {
			if status, ok := event["status"].(string); ok {
				progress <- status
			}
		}
	}

	return nil
}

// ListImages 列出镜像
func (s *Service) ListImages(ctx context.Context) ([]ImageInfo, error) {
	if s.client == nil {
		return nil, errors.New("docker client not available")
	}

	images, err := s.client.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		return nil, err
	}

	result := make([]ImageInfo, 0, len(images))
	for _, img := range images {
		repo := "<none>"
		tag := "<none>"
		if len(img.RepoTags) > 0 {
			parts := strings.SplitN(img.RepoTags[0], ":", 2)
			repo = parts[0]
			if len(parts) > 1 {
				tag = parts[1]
			}
		}
		result = append(result, ImageInfo{
			ID:         img.ID,
			Repository: repo,
			Tag:        tag,
			Size:       img.Size,
			Created:    img.Created,
		})
	}

	return result, nil
}

// RemoveImage 删除镜像
func (s *Service) RemoveImage(ctx context.Context, imageID string) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}
	_, err := s.client.ImageRemove(ctx, imageID, types.ImageRemoveOptions{Force: true, PruneChildren: true})
	return err
}

// ImageExists 检查镜像是否存在
func (s *Service) ImageExists(ctx context.Context, image string) bool {
	if s.client == nil {
		return false
	}
	images, err := s.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return false
	}
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == image || strings.HasPrefix(tag, image+":") {
				return true
			}
		}
	}
	return false
}

// ListContainers 列出容器
func (s *Service) ListContainers(ctx context.Context, all bool) ([]ContainerInfo, error) {
	if s.client == nil {
		return nil, errors.New("docker client not available")
	}

	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{All: all})
	if err != nil {
		return nil, err
	}

	result := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		ports := make([]PortBinding, 0)
		for _, p := range c.Ports {
			ports = append(ports, PortBinding{
				HostPort:      fmt.Sprintf("%d", p.PublicPort),
				ContainerPort: fmt.Sprintf("%d", p.PrivatePort),
				Protocol:      p.Type,
			})
		}

		result = append(result, ContainerInfo{
			ID:      c.ID,
			Name:    name,
			Image:   c.Image,
			ImageID: c.ImageID,
			State:   c.State,
			Status:  c.Status,
			Created: c.Created,
			Ports:   ports,
		})
	}

	return result, nil
}

// CreateContainer 创建容器
func (s *Service) CreateContainer(ctx context.Context, config *ContainerConfig) (string, error) {
	if s.client == nil {
		return "", errors.New("docker client not available")
	}

	// 检查镜像
	if !s.ImageExists(ctx, config.Image) {
		s.logger.Info("Pulling image", zap.String("image", config.Image))
		if err := s.PullImage(ctx, config.Image, nil); err != nil {
			return "", fmt.Errorf("failed to pull image: %w", err)
		}
	}

	// 端口绑定
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for containerPort, hostPort := range config.Ports {
		port := nat.Port(containerPort)
		exposedPorts[port] = struct{}{}
		portBindings[port] = []nat.PortBinding{
			{HostPort: hostPort},
		}
	}

	// 卷挂载
	binds := make([]string, 0)
	for hostPath, containerPath := range config.Volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// 重启策略
	restartPolicy := containertypes.RestartPolicy{}
	if config.Restart != "" {
		restartPolicy.Name = config.Restart
	}

	containerConfig := &containertypes.Config{
		Image:        config.Image,
		Env:          config.Environment,
		ExposedPorts: exposedPorts,
		Labels:       config.Labels,
		Cmd:          config.Command,
	}

	hostConfig := &containertypes.HostConfig{
		PortBindings:  portBindings,
		Binds:         binds,
		RestartPolicy: restartPolicy,
		Privileged:    config.Privileged,
		CapAdd:        config.CapAdd,
	}

	// 设备映射
	if len(config.Devices) > 0 {
		devices := make([]containertypes.DeviceMapping, 0, len(config.Devices))
		for _, d := range config.Devices {
			devices = append(devices, containertypes.DeviceMapping{
				PathOnHost:        d,
				PathInContainer:   d,
				CgroupPermissions: "rwm",
			})
		}
		hostConfig.Devices = devices
	}

	// 网络配置
	networkConfig := &networktypes.NetworkingConfig{}

	resp, err := s.client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, config.Name)
	if err != nil {
		return "", err
	}

	// 启动容器
	if err := s.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		s.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return "", err
	}

	return resp.ID, nil
}

// StartContainer 启动容器
func (s *Service) StartContainer(ctx context.Context, containerID string) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}
	return s.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// StopContainer 停止容器
func (s *Service) StopContainer(ctx context.Context, containerID string) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}
	timeout := 30
	return s.client.ContainerStop(ctx, containerID, containertypes.StopOptions{Timeout: &timeout})
}

// RestartContainer 重启容器
func (s *Service) RestartContainer(ctx context.Context, containerID string) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}
	timeout := 30
	return s.client.ContainerRestart(ctx, containerID, containertypes.StopOptions{Timeout: &timeout})
}

// RemoveContainer 删除容器
func (s *Service) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}
	return s.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force:         force,
		RemoveVolumes: true,
	})
}

// GetContainerStatus 获取容器状态
func (s *Service) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	if s.client == nil {
		return nil, errors.New("docker client not available")
	}

	inspect, err := s.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	return &ContainerStatus{
		ID:        inspect.ID,
		Name:      strings.TrimPrefix(inspect.Name, "/"),
		Image:     inspect.Config.Image,
		State:     inspect.State.Status,
		Running:   inspect.State.Running,
		StartedAt: inspect.State.StartedAt,
		ExitCode:  inspect.State.ExitCode,
	}, nil
}

// GetContainerStats 获取容器统计
func (s *Service) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	if s.client == nil {
		return nil, errors.New("docker client not available")
	}

	resp, err := s.client.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats types.StatsJSON
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	// 计算 CPU 使用率
	cpuPercent := 0.0
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// 计算内存使用率
	memPercent := 0.0
	if stats.MemoryStats.Limit > 0 {
		memPercent = float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100.0
	}

	// 计算网络 IO
	var netRx, netTx uint64
	for _, v := range stats.Networks {
		netRx += v.RxBytes
		netTx += v.TxBytes
	}

	// 计算块 IO
	var blockRead, blockWrite uint64
	if len(stats.BlkioStats.IoServiceBytesRecursive) >= 2 {
		blockRead = stats.BlkioStats.IoServiceBytesRecursive[0].Value
		blockWrite = stats.BlkioStats.IoServiceBytesRecursive[1].Value
	}

	return &ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   stats.MemoryStats.Usage,
		MemoryLimit:   stats.MemoryStats.Limit,
		MemoryPercent: memPercent,
		NetworkRx:     netRx,
		NetworkTx:     netTx,
		BlockRead:     blockRead,
		BlockWrite:    blockWrite,
	}, nil
}

// GetContainerLogs 获取容器日志
func (s *Service) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	if s.client == nil {
		return "", errors.New("docker client not available")
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
		Timestamps: true,
	}

	reader, err := s.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var logs strings.Builder
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// 跳过 Docker 日志流的 8 字节头
		if len(line) > 8 {
			line = line[8:]
		}
		logs.WriteString(line)
		logs.WriteString("\n")
	}

	return logs.String(), nil
}

// ExecInContainer 在容器内执行命令
func (s *Service) ExecInContainer(ctx context.Context, containerID string, command string) (string, error) {
	if s.client == nil {
		return "", errors.New("docker client not available")
	}

	// 创建 exec 实例
	execConfig := types.ExecConfig{
		Cmd:          []string{"/bin/sh", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	execResp, err := s.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// 附加到 exec 实例
	attachResp, err := s.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer attachResp.Close()

	// 读取输出
	var output strings.Builder
	scanner := bufio.NewScanner(attachResp.Reader)
	for scanner.Scan() {
		line := scanner.Text()
		// 跳过 Docker 流的 8 字节头
		if len(line) > 8 {
			line = line[8:]
		}
		output.WriteString(line)
		output.WriteString("\n")
	}

	return output.String(), nil
}

// ListNetworks 列出网络
func (s *Service) ListNetworks(ctx context.Context) ([]NetworkInfo, error) {
	if s.client == nil {
		return nil, errors.New("docker client not available")
	}

	networks, err := s.client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]NetworkInfo, 0, len(networks))
	for _, n := range networks {
		subnet := ""
		gateway := ""
		if len(n.IPAM.Config) > 0 {
			subnet = n.IPAM.Config[0].Subnet
			gateway = n.IPAM.Config[0].Gateway
		}
		result = append(result, NetworkInfo{
			ID:      n.ID,
			Name:    n.Name,
			Driver:  n.Driver,
			Scope:   n.Scope,
			Subnet:  subnet,
			Gateway: gateway,
		})
	}

	return result, nil
}

// CreateNetwork 创建网络
func (s *Service) CreateNetwork(ctx context.Context, name, driver string) (string, error) {
	if s.client == nil {
		return "", errors.New("docker client not available")
	}

	resp, err := s.client.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver: driver,
	})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// RemoveNetwork 删除网络
func (s *Service) RemoveNetwork(ctx context.Context, networkID string) error {
	if s.client == nil {
		return errors.New("docker client not available")
	}
	return s.client.NetworkRemove(ctx, networkID)
}
