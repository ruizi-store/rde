// Package i18n 镜像源配置管理
package i18n

import (
	_ "embed"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed mirrors.yaml
var mirrorsYAML []byte

var (
	mirrorsConfig *MirrorsConfig
	mirrorsOnce   sync.Once
)

// MirrorsConfig 镜像源配置
type MirrorsConfig struct {
	Regions      map[string]RegionMirrors `yaml:"regions" json:"regions"`
	LyricSources map[string][]LyricSource `yaml:"lyric_sources" json:"lyric_sources"`
}

// RegionMirrors 区域镜像配置
type RegionMirrors struct {
	APT     []MirrorEntry `yaml:"apt" json:"apt"`
	Docker  []MirrorEntry `yaml:"docker" json:"docker"`
	NPM     MirrorEntry   `yaml:"npm" json:"npm"`
	PIP     PIPMirror     `yaml:"pip" json:"pip"`
	Flatpak MirrorEntry   `yaml:"flatpak" json:"flatpak"`
	Go      MirrorEntry   `yaml:"go" json:"go"`
	Cargo   MirrorEntry   `yaml:"cargo" json:"cargo"`
	GitHub  MirrorEntry   `yaml:"github" json:"github"`
	Maven   MirrorEntry   `yaml:"maven" json:"maven"`
}

// MirrorEntry 镜像源条目
type MirrorEntry struct {
	Name     string `yaml:"name" json:"name"`
	URL      string `yaml:"url" json:"url"`
	Repo     string `yaml:"repo,omitempty" json:"repo,omitempty"`
	Priority int    `yaml:"priority,omitempty" json:"priority,omitempty"`
}

// PIPMirror PIP 镜像配置
type PIPMirror struct {
	URL         string `yaml:"url" json:"url"`
	TrustedHost string `yaml:"trusted_host,omitempty" json:"trusted_host,omitempty"`
}

// LyricSource 歌词源
type LyricSource struct {
	ID   string            `yaml:"id" json:"id"`
	Name map[string]string `yaml:"name" json:"name"`
}

// GetMirrorsConfig 获取镜像配置（懒加载）
func GetMirrorsConfig() *MirrorsConfig {
	mirrorsOnce.Do(func() {
		mirrorsConfig = &MirrorsConfig{}
		if err := yaml.Unmarshal(mirrorsYAML, mirrorsConfig); err != nil {
			// 使用默认配置
			mirrorsConfig = getDefaultMirrorsConfig()
		}
	})
	return mirrorsConfig
}

// GetMirrorURL 获取指定服务的镜像 URL
// service: apt, docker, npm, pip, flatpak
// setting: follow, cn, intl, custom
// region: 当 setting 为 follow 时使用的区域
// customURL: 当 setting 为 custom 时使用的自定义 URL
func GetMirrorURL(service, setting, region, customURL string) string {
	if setting == MirrorCustom && customURL != "" {
		return customURL
	}

	targetRegion := region
	if setting == MirrorCN {
		targetRegion = RegionCN
	} else if setting == MirrorIntl {
		targetRegion = RegionIntl
	}

	config := GetMirrorsConfig()
	regionConfig, ok := config.Regions[targetRegion]
	if !ok {
		regionConfig = config.Regions[RegionIntl]
	}

	switch service {
	case "apt":
		if len(regionConfig.APT) > 0 {
			return regionConfig.APT[0].URL
		}
	case "docker":
		if len(regionConfig.Docker) > 0 {
			return regionConfig.Docker[0].URL
		}
	case "npm":
		return regionConfig.NPM.URL
	case "pip":
		return regionConfig.PIP.URL
	case "flatpak":
		return regionConfig.Flatpak.URL
	case "go":
		return regionConfig.Go.URL
	case "cargo":
		return regionConfig.Cargo.URL
	case "github":
		return regionConfig.GitHub.URL
	case "maven":
		return regionConfig.Maven.URL
	}

	return ""
}

// GetMirrorField 获取指定服务和字段的镜像配置
// service: apt, docker, npm, pip, flatpak, android
// field: url, repo, docker_image 等
// 使用系统当前区域设置
func GetMirrorField(service, field string) string {
	// 获取当前区域设置（默认为检测的区域）
	region := DetectRegion()

	config := GetMirrorsConfig()
	regionConfig, ok := config.Regions[region]
	if !ok {
		regionConfig = config.Regions[RegionIntl]
	}

	switch service {
	case "apt":
		if len(regionConfig.APT) > 0 {
			if field == "url" || field == "" {
				return regionConfig.APT[0].URL
			}
		}
	case "docker":
		if len(regionConfig.Docker) > 0 {
			if field == "url" || field == "" {
				return regionConfig.Docker[0].URL
			}
		}
	case "npm":
		if field == "url" || field == "" {
			return regionConfig.NPM.URL
		}
	case "pip":
		if field == "url" || field == "" {
			return regionConfig.PIP.URL
		}
		if field == "trusted_host" {
			return regionConfig.PIP.TrustedHost
		}
	case "flatpak":
		if field == "url" || field == "" {
			return regionConfig.Flatpak.URL
		}
		if field == "repo" {
			if regionConfig.Flatpak.Repo != "" {
				return regionConfig.Flatpak.Repo
			}
			return regionConfig.Flatpak.URL
		}
	case "go":
		if field == "url" || field == "" {
			return regionConfig.Go.URL
		}
	case "cargo":
		if field == "url" || field == "" {
			return regionConfig.Cargo.URL
		}
	case "github":
		if field == "url" || field == "" {
			return regionConfig.GitHub.URL
		}
	case "maven":
		if field == "url" || field == "" {
			return regionConfig.Maven.URL
		}
	}

	return ""
}

// GetLyricSources 获取指定区域的歌词源列表
func GetLyricSources(region string) []LyricSource {
	config := GetMirrorsConfig()
	if sources, ok := config.LyricSources[region]; ok {
		return sources
	}
	return config.LyricSources[RegionIntl]
}

// GetAllMirrorOptions 获取所有镜像选项（用于设置界面）
func GetAllMirrorOptions() map[string]map[string][]MirrorEntry {
	config := GetMirrorsConfig()
	result := make(map[string]map[string][]MirrorEntry)

	for regionCode, regionMirrors := range config.Regions {
		result[regionCode] = map[string][]MirrorEntry{
			"apt":     regionMirrors.APT,
			"docker":  regionMirrors.Docker,
			"npm":     {regionMirrors.NPM},
			"pip":     {{Name: regionMirrors.PIP.URL, URL: regionMirrors.PIP.URL}},
			"flatpak": {regionMirrors.Flatpak},
			"go":      {regionMirrors.Go},
			"cargo":   {regionMirrors.Cargo},
			"github":  {regionMirrors.GitHub},
			"maven":   {regionMirrors.Maven},
		}
	}

	return result
}

// MirrorServices 所有支持的镜像服务
var MirrorServices = []struct {
	ID   string
	Name map[string]string
}{
	{ID: "apt", Name: map[string]string{"zh-CN": "APT 软件源", "en-US": "APT Repository"}},
	{ID: "docker", Name: map[string]string{"zh-CN": "Docker 镜像源", "en-US": "Docker Registry"}},
	{ID: "npm", Name: map[string]string{"zh-CN": "NPM 源", "en-US": "NPM Registry"}},
	{ID: "pip", Name: map[string]string{"zh-CN": "PIP 源", "en-US": "PIP Index"}},
	{ID: "flatpak", Name: map[string]string{"zh-CN": "Flatpak 源", "en-US": "Flatpak Repository"}},
	{ID: "go", Name: map[string]string{"zh-CN": "Go 模块代理", "en-US": "Go Module Proxy"}},
	{ID: "cargo", Name: map[string]string{"zh-CN": "Cargo 源", "en-US": "Cargo Registry"}},
	{ID: "github", Name: map[string]string{"zh-CN": "GitHub 加速", "en-US": "GitHub Proxy"}},
	{ID: "maven", Name: map[string]string{"zh-CN": "Maven 仓库", "en-US": "Maven Repository"}},
}

// getDefaultMirrorsConfig 获取默认镜像配置
func getDefaultMirrorsConfig() *MirrorsConfig {
	return &MirrorsConfig{
		Regions: map[string]RegionMirrors{
			RegionCN: {
				APT: []MirrorEntry{
					{Name: "清华大学", URL: "https://mirrors.tuna.tsinghua.edu.cn", Priority: 1},
					{Name: "阿里云", URL: "https://mirrors.aliyun.com", Priority: 2},
				},
				Docker: []MirrorEntry{
					{Name: "阿里云", URL: "https://registry.cn-hangzhou.aliyuncs.com"},
				},
				NPM:     MirrorEntry{Name: "npmmirror", URL: "https://registry.npmmirror.com"},
				PIP:     PIPMirror{URL: "https://pypi.tuna.tsinghua.edu.cn/simple", TrustedHost: "pypi.tuna.tsinghua.edu.cn"},
				Flatpak: MirrorEntry{Name: "上海交大", URL: "https://mirror.sjtu.edu.cn/flathub"},
				Go:      MirrorEntry{Name: "goproxy.cn", URL: "https://goproxy.cn"},
				Cargo:   MirrorEntry{Name: "中科大", URL: "https://mirrors.ustc.edu.cn/crates.io-index"},
				GitHub:  MirrorEntry{Name: "ghproxy", URL: "https://ghproxy.com"},
				Maven:   MirrorEntry{Name: "阿里云", URL: "https://maven.aliyun.com/repository/public"},
			},
			RegionIntl: {
				APT:     []MirrorEntry{{Name: "Official", URL: ""}},
				Docker:  []MirrorEntry{{Name: "Docker Hub", URL: ""}},
				NPM:     MirrorEntry{Name: "npmjs", URL: "https://registry.npmjs.org"},
				PIP:     PIPMirror{URL: "https://pypi.org/simple"},
				Flatpak: MirrorEntry{Name: "Flathub", URL: "https://flathub.org/repo"},
				Go:      MirrorEntry{Name: "proxy.golang.org", URL: "https://proxy.golang.org"},
				Cargo:   MirrorEntry{Name: "crates.io", URL: "https://crates.io"},
				GitHub:  MirrorEntry{Name: "GitHub", URL: "https://github.com"},
				Maven:   MirrorEntry{Name: "Maven Central", URL: "https://repo1.maven.org/maven2"},
			},
		},
		LyricSources: map[string][]LyricSource{
			RegionCN: {
				{ID: "netease", Name: map[string]string{"zh-CN": "网易云音乐", "en-US": "NetEase Music"}},
				{ID: "qq", Name: map[string]string{"zh-CN": "QQ音乐", "en-US": "QQ Music"}},
			},
			RegionIntl: {
				{ID: "genius", Name: map[string]string{"zh-CN": "Genius", "en-US": "Genius"}},
				{ID: "musixmatch", Name: map[string]string{"zh-CN": "Musixmatch", "en-US": "Musixmatch"}},
			},
		},
	}
}
