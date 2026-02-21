package retrogame

// SetupStatus 表示 EmulatorJS 的安装状态
type SetupStatus struct {
	Installed   bool   `json:"installed"`
	Version     string `json:"version"`
	EmulatorDir string `json:"emulator_dir"`
}

// ProgressEvent SSE 进度事件
type ProgressEvent struct {
	Status   string `json:"status"`   // downloading, extracting, completed, failed
	Message  string `json:"message"`
	Progress int    `json:"progress"` // 0-100
}

// RomFileInfo 扫描到的 ROM 文件信息
type RomFileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Platform string `json:"platform"`
}
