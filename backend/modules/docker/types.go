// Package docker Docker 类型定义
package docker

// ContainerConfig 容器创建配置
type ContainerConfig struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       map[string]string `json:"ports"`
	Volumes     map[string]string `json:"volumes"`
	Environment []string          `json:"environment"`
	Networks    []string          `json:"networks"`
	Labels      map[string]string `json:"labels"`
	Restart     string            `json:"restart"`
	Privileged  bool              `json:"privileged"`
	CapAdd      []string          `json:"cap_add"`
	Devices     []string          `json:"devices"`
	Command     []string          `json:"command"`
}

// ContainerStatus 容器状态
type ContainerStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	State     string `json:"state"`
	Running   bool   `json:"running"`
	StartedAt string `json:"started_at"`
	ExitCode  int    `json:"exit_code"`
}

// ContainerStats 容器资源统计
type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
	BlockRead     uint64  `json:"block_read"`
	BlockWrite    uint64  `json:"block_write"`
}

// PortBinding 端口绑定
type PortBinding struct {
	HostPort      string `json:"host_port"`
	ContainerPort string `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// ContainerInfo 容器简要信息
type ContainerInfo struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Image   string        `json:"image"`
	ImageID string        `json:"image_id"`
	State   string        `json:"state"`
	Status  string        `json:"status"`
	Created int64         `json:"created"`
	Ports   []PortBinding `json:"ports"`
}

// ImageInfo 镜像信息
type ImageInfo struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       int64  `json:"size"`
	Created    int64  `json:"created"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Scope   string `json:"scope"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// DockerInfo Docker 系统信息
type DockerInfo struct {
	Version           string `json:"version"`
	APIVersion        string `json:"api_version"`
	OS                string `json:"os"`
	Arch              string `json:"arch"`
	KernelVersion     string `json:"kernel_version"`
	Containers        int    `json:"containers"`
	ContainersRunning int    `json:"containers_running"`
	ContainersStopped int    `json:"containers_stopped"`
	Images            int    `json:"images"`
	Driver            string `json:"driver"`
	MemTotal          int64  `json:"mem_total"`
	NCPU              int    `json:"ncpu"`
}

// CreateContainerRequest 创建容器请求
type CreateContainerRequest struct {
	Name        string            `json:"name" binding:"required"`
	Image       string            `json:"image" binding:"required"`
	Ports       map[string]string `json:"ports"`
	Volumes     map[string]string `json:"volumes"`
	Environment []string          `json:"environment"`
	Networks    []string          `json:"networks"`
	Labels      map[string]string `json:"labels"`
	Restart     string            `json:"restart"`
	Privileged  bool              `json:"privileged"`
	CapAdd      []string          `json:"cap_add"`
	Devices     []string          `json:"devices"`
	Command     []string          `json:"command"`
}

// PullImageRequest 拉取镜像请求
type PullImageRequest struct {
	Image string `json:"image" binding:"required"`
}

// CreateNetworkRequest 创建网络请求
type CreateNetworkRequest struct {
	Name   string `json:"name" binding:"required"`
	Driver string `json:"driver"`
}
