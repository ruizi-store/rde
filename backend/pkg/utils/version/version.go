package version

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ruizi-store/rde/backend/common"
)

// VersionInfo 版本信息
type VersionInfo struct {
	Version     string `json:"version"`
	ReleaseDate string `json:"release_date"`
	DownloadURL string `json:"download_url"`
	Changelog   string `json:"changelog"`
}

// IsNeedUpdate 检查是否需要更新
func IsNeedUpdate(currentVersion string) (bool, VersionInfo) {
	latestVersion := GetLatestVersion()
	if latestVersion.Version == "" {
		return false, VersionInfo{}
	}

	// 比较版本号
	if CompareVersion(latestVersion.Version, currentVersion) > 0 {
		return true, latestVersion
	}

	return false, VersionInfo{}
}

// GetLatestVersion 获取最新版本信息
func GetLatestVersion() VersionInfo {
	// 从远程获取最新版本信息
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(common.REMOTE_VERSION_URL)
	if err != nil {
		return VersionInfo{}
	}
	defer resp.Body.Close()

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return VersionInfo{}
	}

	return info
}

// CompareVersion 比较版本号
// 返回: 1 表示 v1 > v2, -1 表示 v1 < v2, 0 表示相等
func CompareVersion(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parseVersionPart(parts1[i])
		}
		if i < len(parts2) {
			p2 = parseVersionPart(parts2[i])
		}

		if p1 > p2 {
			return 1
		}
		if p1 < p2 {
			return -1
		}
	}

	return 0
}

func parseVersionPart(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
