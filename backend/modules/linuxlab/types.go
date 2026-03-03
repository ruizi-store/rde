package linuxlab

// Board 开发板信息
type Board struct {
	Arch      string `json:"arch"`
	Name      string `json:"name"`
	FullPath  string `json:"full_path"`
	CPU       string `json:"cpu"`
	MEM       string `json:"mem"`
	SMP       int    `json:"smp"`
	Linux     string `json:"linux"`
	QEMU      string `json:"qemu"`
	UBoot     string `json:"uboot"`
	Buildroot string `json:"buildroot"`
	NetDev    string `json:"netdev"`
	Serial    string `json:"serial"`
	RootDev   string `json:"rootdev"`
}

// LabStatus 实验环境状态（容器模式）
type LabStatus struct {
	DockerOK         bool   `json:"docker_ok"`
	ImageReady       bool   `json:"image_ready"`
	ContainerRunning bool   `json:"container_running"`
	ContainerExists  bool   `json:"container_exists"`
	CurrentBoard     string `json:"current_board"`
	Building         bool   `json:"building"`
	Booting          bool   `json:"booting"`
	Image            string `json:"image"`
}

// BuildRequest 构建请求
type BuildRequest struct {
	Board  string `json:"board"`
	Target string `json:"target"`
}

// BootRequest 启动请求
type BootRequest struct {
	Board string `json:"board"`
}

// MakeRequest 执行任意 make 目标
type MakeRequest struct {
	Board  string `json:"board"`
	Target string `json:"target"`
}

// SwitchBoardRequest 切换开发板请求
type SwitchBoardRequest struct {
	Board string `json:"board" binding:"required"`
}

// ProgressEvent SSE 进度事件
type ProgressEvent struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Line    string `json:"line"`
}
