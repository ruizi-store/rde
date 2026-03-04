package cloud_backup

import "io"

// CloudTargetConfig 云备份目标配置
type CloudTargetConfig struct {
	CloudToken string `json:"cloud_token"` // 云端 JWT Token
	CloudURL   string `json:"cloud_url"`   // 云端 API 地址
}

// TargetTestResponse 测试目标连接响应
type TargetTestResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	FreeSpace int64  `json:"free_space,omitempty"`
}

// RemoteFile 远程文件信息
type RemoteFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

// ProgressReader 带进度回调的 Reader
type ProgressReader struct {
	Reader   io.Reader
	Total    int64
	Current  int64
	Callback func(int)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Current += int64(n)
		if pr.Callback != nil && pr.Total > 0 {
			progress := int(pr.Current * 100 / pr.Total)
			pr.Callback(progress)
		}
	}
	return n, err
}
