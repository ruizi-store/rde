// Package flatpak KasmVNC 自动安装/升级管理
package flatpak

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/ruizi-store/rde/backend/core/i18n"
	"go.uber.org/zap"
)

// distroInfo 系统发行版信息
type distroInfo struct {
	ID            string // debian, ubuntu, fedora, opensuse-leap, alpine, ol (oracle)
	VersionID     string // 12, 24.04, 40, 15, 3.20 等
	VersionCodename string // bookworm, noble 等（仅 Debian/Ubuntu）
}

// Installer KasmVNC 安装管理器
type Installer struct {
	logger *zap.Logger
}

// NewInstaller 创建安装管理器
func NewInstaller(logger *zap.Logger) *Installer {
	return &Installer{logger: logger}
}

// IsInstalled 检查 KasmVNC 是否已安装（通过系统包管理器）
func (inst *Installer) IsInstalled() bool {
	// 优先检查 dpkg
	if out, err := exec.Command("dpkg", "-s", "kasmvncserver").Output(); err == nil {
		if strings.Contains(string(out), "Status: install ok installed") {
			return true
		}
	}
	// rpm
	if err := exec.Command("rpm", "-q", "kasmvncserver").Run(); err == nil {
		return true
	}
	// 检查二进制存在
	for _, p := range []string{"/usr/bin/Xkasmvnc", "/usr/local/bin/Xkasmvnc", "/usr/bin/kasmvncserver"} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// GetInstalledVersion 获取已安装的版本
func (inst *Installer) GetInstalledVersion() string {
	// dpkg
	if out, err := exec.Command("dpkg-query", "-W", "-f=${Version}", "kasmvncserver").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			return v
		}
	}
	// rpm
	if out, err := exec.Command("rpm", "-q", "--qf", "%{VERSION}", "kasmvncserver").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			return v
		}
	}
	return ""
}

// NeedsUpdate 检查是否需要更新
func (inst *Installer) NeedsUpdate() bool {
	if !inst.IsInstalled() {
		return true
	}
	installed := inst.GetInstalledVersion()
	return installed != kasmVNCVersion
}

// detectDistro 从 /etc/os-release 检测发行版信息
func detectDistro() (*distroInfo, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, fmt.Errorf("无法读取 /etc/os-release: %w", err)
	}
	defer f.Close()

	info := &distroInfo{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch k {
		case "ID":
			info.ID = v
		case "VERSION_ID":
			info.VersionID = v
		case "VERSION_CODENAME":
			info.VersionCodename = v
		}
	}

	if info.ID == "" {
		return nil, fmt.Errorf("无法识别发行版")
	}
	return info, nil
}

// codename2kasmName 将发行版信息映射为 KasmVNC release 中使用的名称
// 返回 (名称, 包格式)
func codename2kasmName(d *distroInfo) (string, string, error) {
	arch := runtime.GOARCH

	switch d.ID {
	case "debian":
		codename := d.VersionCodename
		if codename == "" {
			// 从版本号推断
			switch {
			case strings.HasPrefix(d.VersionID, "13"):
				codename = "trixie"
			case strings.HasPrefix(d.VersionID, "12"):
				codename = "bookworm"
			case strings.HasPrefix(d.VersionID, "11"):
				codename = "bullseye"
			case strings.HasPrefix(d.VersionID, "10"):
				codename = "buster"
			}
		}
		if codename == "" {
			return "", "", fmt.Errorf("不支持的 Debian 版本: %s", d.VersionID)
		}
		debArch := "amd64"
		if arch == "arm64" {
			debArch = "arm64"
		}
		return fmt.Sprintf("kasmvncserver_%s_%s_%s.deb", codename, kasmVNCVersion, debArch), "deb", nil

	case "ubuntu", "linuxmint", "pop":
		codename := d.VersionCodename
		if codename == "" {
			// Ubuntu 版本号 → codename 映射
			switch {
			case strings.HasPrefix(d.VersionID, "24.04"):
				codename = "noble"
			case strings.HasPrefix(d.VersionID, "22.04"):
				codename = "jammy"
			case strings.HasPrefix(d.VersionID, "20.04"):
				codename = "focal"
			}
		}
		// Linux Mint / Pop!_OS 使用 Ubuntu 基础
		if d.ID == "linuxmint" || d.ID == "pop" {
			switch {
			case strings.HasPrefix(d.VersionID, "22") || strings.HasPrefix(d.VersionID, "21."):
				codename = "jammy"
			case strings.HasPrefix(d.VersionID, "20"):
				codename = "focal"
			default:
				codename = "noble"
			}
		}
		if codename == "" {
			return "", "", fmt.Errorf("不支持的 Ubuntu 版本: %s", d.VersionID)
		}
		debArch := "amd64"
		if arch == "arm64" {
			debArch = "arm64"
		}
		return fmt.Sprintf("kasmvncserver_%s_%s_%s.deb", codename, kasmVNCVersion, debArch), "deb", nil

	case "kali":
		debArch := "amd64"
		if arch == "arm64" {
			debArch = "arm64"
		}
		return fmt.Sprintf("kasmvncserver_kali-rolling_%s_%s.deb", kasmVNCVersion, debArch), "deb", nil

	case "fedora":
		// Fedora 版本号 → 英文名映射
		fedoraName := ""
		switch d.VersionID {
		case "41":
			fedoraName = "fortyone"
		case "40":
			fedoraName = "forty"
		case "39":
			fedoraName = "thirtynine"
		default:
			fedoraName = "fortyone" // 默认使用最新
		}
		rpmArch := "x86_64"
		if arch == "arm64" {
			rpmArch = "aarch64"
		}
		return fmt.Sprintf("kasmvncserver_fedora_%s_%s_%s.rpm", fedoraName, kasmVNCVersion, rpmArch), "rpm", nil

	case "opensuse-leap", "opensuse-tumbleweed":
		rpmArch := "x86_64"
		if arch == "arm64" {
			rpmArch = "aarch64"
		}
		return fmt.Sprintf("kasmvncserver_opensuse_15_%s_%s.rpm", kasmVNCVersion, rpmArch), "rpm", nil

	case "ol", "oracle", "rocky", "almalinux", "centos", "rhel":
		rpmArch := "x86_64"
		if arch == "arm64" {
			rpmArch = "aarch64"
		}
		ver := "9"
		if strings.HasPrefix(d.VersionID, "8") {
			ver = "8"
		}
		return fmt.Sprintf("kasmvncserver_oracle_%s_%s_%s.rpm", ver, kasmVNCVersion, rpmArch), "rpm", nil

	case "alpine":
		alpineVer := strings.ReplaceAll(d.VersionID, ".", "")
		if len(alpineVer) > 3 {
			alpineVer = alpineVer[:3]
		}
		apkArch := "x86_64"
		if arch == "arm64" {
			apkArch = "aarch64"
		}
		return fmt.Sprintf("kasmvnc.alpine_%s_%s.tgz", alpineVer, apkArch), "alpine", nil
	}

	return "", "", fmt.Errorf("不支持的发行版: %s %s", d.ID, d.VersionID)
}

// GetDownloadURL 获取下载地址（国内自动走 ghproxy 加速）
func (inst *Installer) GetDownloadURL() (string, string, error) {
	distro, err := detectDistro()
	if err != nil {
		return "", "", err
	}

	filename, pkgType, err := codename2kasmName(distro)
	if err != nil {
		return "", "", err
	}

	base := "https://github.com/kasmtech/KasmVNC/releases/download"
	directURL := fmt.Sprintf("%s/v%s/%s", base, kasmVNCVersion, filename)

	// 检测是否使用国内镜像加速
	ghProxy := i18n.GetMirrorField("github", "url")
	if ghProxy != "" && ghProxy != "https://github.com" {
		return fmt.Sprintf("%s/%s", ghProxy, directURL), pkgType, nil
	}
	return directURL, pkgType, nil
}

// Install 下载并安装 KasmVNC
func (inst *Installer) Install(onProgress func(line string)) error {
	downloadURL, pkgType, err := inst.GetDownloadURL()
	if err != nil {
		return fmt.Errorf("detect distro: %w", err)
	}

	inst.logger.Info("downloading KasmVNC",
		zap.String("url", downloadURL),
		zap.String("version", kasmVNCVersion),
		zap.String("pkgType", pkgType),
	)

	onProgress(fmt.Sprintf("正在下载 KasmVNC v%s ...", kasmVNCVersion))
	onProgress(fmt.Sprintf("下载地址: %s", downloadURL))

	// 下载
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download KasmVNC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download KasmVNC: HTTP %d", resp.StatusCode)
	}

	// 确定临时文件后缀
	suffix := ".deb"
	switch pkgType {
	case "rpm":
		suffix = ".rpm"
	case "alpine":
		suffix = ".tgz"
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "kasmvnc-*"+suffix)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// 写入临时文件
	written, err := io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("save download: %w", err)
	}
	onProgress(fmt.Sprintf("下载完成，大小: %.1f MB", float64(written)/1024/1024))

	// 根据包类型安装
	onProgress("正在安装...")
	switch pkgType {
	case "deb":
		// 先安装依赖
		cmd := exec.Command("apt-get", "install", "-y", "-f", tmpPath)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		out, err := cmd.CombinedOutput()
		if err != nil {
			// 回退到 dpkg + apt-get fix
			onProgress("使用 dpkg 安装...")
			dpkgCmd := exec.Command("dpkg", "-i", tmpPath)
			dpkgOut, dpkgErr := dpkgCmd.CombinedOutput()
			if dpkgErr != nil {
				onProgress(fmt.Sprintf("dpkg 输出: %s", string(dpkgOut)))
				// 尝试修复依赖
				fixCmd := exec.Command("apt-get", "install", "-y", "-f")
				fixCmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
				fixOut, fixErr := fixCmd.CombinedOutput()
				if fixErr != nil {
					return fmt.Errorf("install KasmVNC deb: %s\n%s", string(dpkgOut), string(fixOut))
				}
				onProgress("依赖修复完成")
			}
		} else {
			// 将安装输出中有用的信息传给前端
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && (strings.Contains(line, "kasmvnc") || strings.Contains(line, "Setting up") || strings.Contains(line, "installed")) {
					onProgress(line)
				}
			}
		}

	case "rpm":
		cmd := exec.Command("rpm", "-Uvh", "--force", tmpPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install KasmVNC rpm: %s", string(out))
		}
		onProgress(string(out))

	case "alpine":
		// Alpine 使用 tgz，解压到 /
		cmd := exec.Command("tar", "-xzf", tmpPath, "-C", "/")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("install KasmVNC alpine: %s", string(out))
		}
		onProgress("解压完成")

	default:
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}

	onProgress(fmt.Sprintf("KasmVNC v%s 安装完成", kasmVNCVersion))
	inst.logger.Info("KasmVNC installed", zap.String("version", kasmVNCVersion))
	return nil
}

// GetBinaryPath 获取 kasmvncserver / Xkasmvnc 可执行文件路径
func (inst *Installer) GetBinaryPath() string {
	// 系统包安装后放在标准路径
	for _, p := range []string{
		"/usr/bin/Xkasmvnc",
		"/usr/local/bin/Xkasmvnc",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "Xkasmvnc" // fallback: 依赖 PATH
}

// GetVNCServerPath 获取 kasmvncserver 脚本路径
func (inst *Installer) GetVNCServerPath() string {
	for _, p := range []string{
		"/usr/bin/kasmvncserver",
		"/usr/local/bin/kasmvncserver",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "kasmvncserver"
}

// GetWebDir 获取 Web 客户端目录
func (inst *Installer) GetWebDir() string {
	for _, p := range []string{
		"/usr/share/kasmvnc/www",
		"/usr/local/share/kasmvnc/www",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/share/kasmvnc/www"
}

// CheckSystemDeps 检查系统依赖
func (inst *Installer) CheckSystemDeps() *SetupStatus {
	status := &SetupStatus{
		KasmVNCInstalled: inst.IsInstalled(),
		KasmVNCVersion:   inst.GetInstalledVersion(),
		KasmVNCExpected:  kasmVNCVersion,
	}

	// flatpak
	if _, err := exec.LookPath("flatpak"); err == nil {
		status.FlatpakInstalled = true
		// 检查 flathub remote
		out, err := exec.Command("flatpak", "remotes", "--columns=name").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) == "flathub" {
					status.FlatpakRemoteOK = true
					break
				}
			}
		}
	}

	// openbox
	if _, err := exec.LookPath("openbox"); err == nil {
		status.OpenboxInstalled = true
	}

	// pulseaudio
	if _, err := exec.LookPath("pulseaudio"); err == nil {
		status.PulseAudioInstalled = true

		// 检查是否运行中
		if exec.Command("pulseaudio", "--check").Run() == nil {
			status.PulseAudioRunning = true
		} else if exec.Command("systemctl", "is-active", "--quiet", "pulseaudio-system.service").Run() == nil {
			status.PulseAudioRunning = true
		}
	}

	// 虚拟声卡
	if status.PulseAudioRunning {
		out, err := exec.Command("pactl", "list", "sinks", "short").Output()
		if err == nil && strings.Contains(string(out), "virtual_speaker") {
			status.VirtualSinkReady = true
		}
	}

	status.Ready = status.KasmVNCInstalled &&
		status.FlatpakInstalled &&
		status.FlatpakRemoteOK &&
		status.OpenboxInstalled &&
		status.PulseAudioInstalled &&
		status.PulseAudioRunning &&
		status.VirtualSinkReady

	return status
}
