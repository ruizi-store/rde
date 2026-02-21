package backup

import (
	"io"
)

// Target 备份目标接口
type Target interface {
	// Configure 配置目标（传入 JSON 配置字符串）
	Configure(config string) error

	// Test 测试连接
	Test() *TargetTestResponse

	// Upload 上传文件，返回远程路径
	Upload(localPath, remoteName string, progress func(int)) (string, error)

	// Download 下载文件
	Download(remotePath, localPath string, progress func(int)) error

	// Delete 删除远程文件
	Delete(remotePath string) error

	// List 列出备份文件
	List() ([]RemoteFile, error)
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
