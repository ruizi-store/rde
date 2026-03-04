// Package ai 系统告警服务
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AlertService 告警服务
type AlertService struct {
	logger     *zap.Logger
	dataDir    string
	config     *AlertConfig
	configFile string
	alerts     []Alert
	alertsMu   sync.RWMutex
	cooldowns  map[AlertType]time.Time
	gateway    *GatewayService

	ctx    context.Context
	cancel context.CancelFunc
}

// NewAlertService 创建告警服务
func NewAlertService(logger *zap.Logger, dataDir string, gateway *GatewayService) *AlertService {
	s := &AlertService{
		logger:     logger,
		dataDir:    dataDir,
		configFile: filepath.Join(dataDir, "alerts.json"),
		alerts:     []Alert{},
		cooldowns:  make(map[AlertType]time.Time),
		gateway:    gateway,
	}
	s.loadConfig()
	s.loadAlerts()
	return s
}

func (s *AlertService) loadConfig() {
	s.config = &AlertConfig{
		Enabled:          false,
		CheckInterval:    300,
		DiskWarningPct:   80,
		DiskCriticalPct:  95,
		CPUWarningPct:    90,
		MemoryWarningPct: 90,
		TempWarningC:     70,
		EnabledAlerts:    []AlertType{AlertDiskFull, AlertDiskWarning, AlertContainerDown, AlertHighCPU, AlertHighMemory},
		NotifyPlatforms:  []PlatformType{},
		NotifyUsers:      []string{},
		QuietHoursStart:  -1,
		QuietHoursEnd:    -1,
		CooldownMinutes:  30,
	}
	data, err := os.ReadFile(s.configFile)
	if err == nil {
		json.Unmarshal(data, s.config)
	}
}

func (s *AlertService) saveConfig() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.configFile, data, 0644)
}

func (s *AlertService) loadAlerts() {
	alertsFile := filepath.Join(s.dataDir, "alert_history.json")
	data, err := os.ReadFile(alertsFile)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.alerts); err != nil {
		s.logger.Warn("failed to parse alert history", zap.Error(err))
	}
}

func (s *AlertService) saveAlerts() {
	alertsFile := filepath.Join(s.dataDir, "alert_history.json")
	data, err := json.MarshalIndent(s.alerts, "", "  ")
	if err != nil {
		s.logger.Error("failed to marshal alerts", zap.Error(err))
		return
	}
	tmpFile := alertsFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		s.logger.Error("failed to write alerts", zap.Error(err))
		return
	}
	os.Rename(tmpFile, alertsFile)
}

// GetConfig 获取配置
func (s *AlertService) GetConfig() *AlertConfig { return s.config }

// UpdateConfig 更新配置
func (s *AlertService) UpdateConfig(config *AlertConfig) error {
	s.config = config
	return s.saveConfig()
}

// GetAlerts 获取告警历史
func (s *AlertService) GetAlerts() []Alert {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	return s.alerts
}

// ClearAlerts 清除告警历史
func (s *AlertService) ClearAlerts() {
	s.alertsMu.Lock()
	s.alerts = []Alert{}
	s.alertsMu.Unlock()
	s.saveAlerts()
}

// GetStatus 获取告警状态
func (s *AlertService) GetStatus() map[string]interface{} {
	s.alertsMu.RLock()
	unresolved := 0
	for _, a := range s.alerts {
		if !a.Resolved {
			unresolved++
		}
	}
	total := len(s.alerts)
	s.alertsMu.RUnlock()
	return map[string]interface{}{
		"enabled":         s.config.Enabled,
		"check_interval":  s.config.CheckInterval,
		"total_alerts":    total,
		"unresolved":      unresolved,
		"enabled_alerts":  s.config.EnabledAlerts,
		"notify_platforms": s.config.NotifyPlatforms,
	}
}

// Start 启动监控循环
func (s *AlertService) Start() {
	if !s.config.Enabled {
		return
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	go s.monitorLoop()
	s.logger.Info("Alert service started", zap.Int("interval", s.config.CheckInterval))
}

// Stop 停止监控
func (s *AlertService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.saveAlerts()
}

func (s *AlertService) monitorLoop() {
	interval := time.Duration(s.config.CheckInterval) * time.Second
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.runChecks()
		}
	}
}

func (s *AlertService) runChecks() {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Alert check panic", zap.Any("recover", r))
		}
	}()

	for _, alertType := range s.config.EnabledAlerts {
		switch alertType {
		case AlertDiskFull, AlertDiskWarning:
			s.checkDisk()
		case AlertContainerDown:
			s.checkContainers()
		case AlertHighCPU:
			s.checkCPU()
		case AlertHighMemory:
			s.checkMemory()
		case AlertHighTemp:
			s.checkTemperature()
		case AlertSmartWarning:
			s.checkSmart()
		case AlertRaidDegraded:
			s.checkRaid()
		}
	}
}

func (s *AlertService) isAlertEnabled(t AlertType) bool {
	for _, at := range s.config.EnabledAlerts {
		if at == t {
			return true
		}
	}
	return false
}

func (s *AlertService) isInCooldown(t AlertType) bool {
	s.alertsMu.RLock()
	lastTime, ok := s.cooldowns[t]
	s.alertsMu.RUnlock()
	if ok {
		return time.Since(lastTime) < time.Duration(s.config.CooldownMinutes)*time.Minute
	}
	return false
}

func (s *AlertService) isQuietHours() bool {
	if s.config.QuietHoursStart < 0 || s.config.QuietHoursEnd < 0 {
		return false
	}
	hour := time.Now().Hour()
	start, end := s.config.QuietHoursStart, s.config.QuietHoursEnd
	if start <= end {
		return hour >= start && hour < end
	}
	return hour >= start || hour < end
}

func (s *AlertService) addAlert(alert Alert) {
	if s.isInCooldown(alert.Type) {
		return
	}

	s.alertsMu.Lock()
	s.alerts = append(s.alerts, alert)
	if len(s.alerts) > 1000 {
		s.alerts = s.alerts[len(s.alerts)-500:]
	}
	s.alertsMu.Unlock()

	s.alertsMu.Lock()
	s.cooldowns[alert.Type] = time.Now()
	s.alertsMu.Unlock()

	s.saveAlerts()

	if !s.isQuietHours() || alert.Level == AlertLevelCritical {
		go s.sendNotification(alert)
	}
}

// resolveAlert 自动解除指定类型和来源的未解决告警
func (s *AlertService) resolveAlert(alertType AlertType, source string) {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()

	now := time.Now()
	changed := false
	for i := range s.alerts {
		if s.alerts[i].Type == alertType && s.alerts[i].Source == source && !s.alerts[i].Resolved {
			s.alerts[i].Resolved = true
			s.alerts[i].ResolvedAt = &now
			changed = true
		}
	}
	if changed {
		go s.saveAlerts()
	}
}

func (s *AlertService) sendNotification(alert Alert) {
	if s.gateway == nil {
		return
	}

	emoji := "ℹ️"
	switch alert.Level {
	case AlertLevelWarning:
		emoji = "⚠️"
	case AlertLevelCritical:
		emoji = "🚨"
	}

	text := fmt.Sprintf("%s %s\n\n%s\n\n来源: %s | 值: %.1f | 阈值: %.1f",
		emoji, alert.Title, alert.Message, alert.Source, alert.Value, alert.Threshold)

	for _, platform := range s.config.NotifyPlatforms {
		adapter := s.gateway.GetAdapter(platform)
		if adapter == nil || !adapter.IsEnabled() {
			continue
		}
		for _, userID := range s.config.NotifyUsers {
			msg := OutgoingMessage{Platform: platform, UserID: userID, ChatID: userID, Text: text}
			if err := adapter.SendMessage(context.Background(), msg); err != nil {
				s.logger.Error("Failed to send alert notification",
					zap.String("platform", string(platform)), zap.Error(err))
			}
		}
	}
}

// checkDisk 检查磁盘使用
func (s *AlertService) checkDisk() {
	output, err := exec.Command("df", "-h", "--output=source,pcent,target").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.HasPrefix(fields[0], "/dev/") {
			continue
		}
		pctStr := strings.TrimSuffix(fields[1], "%")
		pct, _ := strconv.ParseFloat(pctStr, 64)
		mount := fields[2]

		if pct >= s.config.DiskCriticalPct && s.isAlertEnabled(AlertDiskFull) {
			s.addAlert(Alert{
				ID: fmt.Sprintf("disk_full_%s_%d", mount, time.Now().Unix()),
				Type: AlertDiskFull, Level: AlertLevelCritical,
				Title: "磁盘空间严重不足", Message: fmt.Sprintf("分区 %s 使用率 %.0f%%", mount, pct),
				Source: mount, Value: pct, Threshold: s.config.DiskCriticalPct,
				Timestamp: time.Now(),
			})
		} else if pct >= s.config.DiskWarningPct && s.isAlertEnabled(AlertDiskWarning) {
			s.addAlert(Alert{
				ID: fmt.Sprintf("disk_warn_%s_%d", mount, time.Now().Unix()),
				Type: AlertDiskWarning, Level: AlertLevelWarning,
				Title: "磁盘空间不足", Message: fmt.Sprintf("分区 %s 使用率 %.0f%%", mount, pct),
				Source: mount, Value: pct, Threshold: s.config.DiskWarningPct,
				Timestamp: time.Now(),
			})
		} else {
			// 恢复正常，自动解除同源告警
			s.resolveAlert(AlertDiskFull, mount)
			s.resolveAlert(AlertDiskWarning, mount)
		}
	}
}

// checkContainers 检查容器状态
func (s *AlertService) checkContainers() {
	output, err := exec.Command("docker", "ps", "-a",
		"--format", "{{.Names}}\t{{.Status}}\t{{.State}}").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		name, state := fields[0], fields[2]
		if state == "exited" || state == "dead" {
			s.addAlert(Alert{
				ID:    fmt.Sprintf("container_%s_%d", name, time.Now().Unix()),
				Type:  AlertContainerDown, Level: AlertLevelWarning,
				Title: "容器异常", Message: fmt.Sprintf("容器 %s 已停止 (状态: %s)", name, state),
				Source: name, Timestamp: time.Now(),
			})
		}
	}
}

// checkCPU 检查 CPU 使用率
func (s *AlertService) checkCPU() {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return
	}
	var total, idle float64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseFloat(fields[i], 64)
		total += v
		if i == 4 {
			idle = v
		}
	}
	usage := 100 * (1 - idle/total)

	if usage >= s.config.CPUWarningPct {
		s.addAlert(Alert{
			ID: fmt.Sprintf("cpu_%d", time.Now().Unix()),
			Type: AlertHighCPU, Level: AlertLevelWarning,
			Title: "CPU 使用率过高", Message: fmt.Sprintf("CPU 使用率 %.1f%%", usage),
			Source: "cpu", Value: usage, Threshold: s.config.CPUWarningPct,
			Timestamp: time.Now(),
		})
	} else {
		s.resolveAlert(AlertHighCPU, "cpu")
	}
}

// checkMemory 检查内存使用率
func (s *AlertService) checkMemory() {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	info := make(map[string]float64)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.TrimSuffix(val, " kB")
		v, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		info[key] = v
	}

	total := info["MemTotal"]
	available := info["MemAvailable"]
	if total <= 0 {
		return
	}
	usage := 100 * (1 - available/total)

	if usage >= s.config.MemoryWarningPct {
		s.addAlert(Alert{
			ID: fmt.Sprintf("memory_%d", time.Now().Unix()),
			Type: AlertHighMemory, Level: AlertLevelWarning,
			Title: "内存使用率过高", Message: fmt.Sprintf("内存使用率 %.1f%%", usage),
			Source: "memory", Value: usage, Threshold: s.config.MemoryWarningPct,
			Timestamp: time.Now(),
		})
	} else {
		s.resolveAlert(AlertHighMemory, "memory")
	}
}

// checkTemperature 检查温度
func (s *AlertService) checkTemperature() {
	output, err := exec.Command("sensors", "-j").Output()
	if err != nil {
		return
	}
	var sensors map[string]interface{}
	if err := json.Unmarshal(output, &sensors); err != nil {
		return
	}
	maxTemp := 0.0
	source := ""
	for chipName, chipData := range sensors {
		chipMap, ok := chipData.(map[string]interface{})
		if !ok {
			continue
		}
		for sensorName, sensorData := range chipMap {
			sensorMap, ok := sensorData.(map[string]interface{})
			if !ok {
				continue
			}
			for key, val := range sensorMap {
				if strings.Contains(key, "input") {
					if temp, ok := val.(float64); ok && temp > maxTemp {
						maxTemp = temp
						source = fmt.Sprintf("%s/%s", chipName, sensorName)
					}
				}
			}
		}
	}
	if maxTemp >= s.config.TempWarningC {
		s.addAlert(Alert{
			ID: fmt.Sprintf("temp_%d", time.Now().Unix()),
			Type: AlertHighTemp, Level: AlertLevelWarning,
			Title: "温度过高", Message: fmt.Sprintf("传感器温度 %.1f°C", maxTemp),
			Source: source, Value: maxTemp, Threshold: s.config.TempWarningC,
			Timestamp: time.Now(),
		})
	} else if source != "" {
		s.resolveAlert(AlertHighTemp, source)
	}
}

// checkSmart 检查 SMART 状态
func (s *AlertService) checkSmart() {
	output, err := exec.Command("lsblk", "-d", "-n", "-o", "NAME,TYPE").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != "disk" {
			continue
		}
		dev := "/dev/" + fields[0]
		smartOutput, err := exec.Command("smartctl", "-H", dev).Output()
		if err != nil {
			continue
		}
		result := string(smartOutput)
		if strings.Contains(result, "FAILED") || strings.Contains(result, "FAILING") {
			s.addAlert(Alert{
				ID: fmt.Sprintf("smart_%s_%d", fields[0], time.Now().Unix()),
				Type: AlertSmartWarning, Level: AlertLevelCritical,
				Title: "SMART 健康警告", Message: fmt.Sprintf("磁盘 %s SMART 检测异常", dev),
				Source: dev, Timestamp: time.Now(),
			})
		}
	}
}

// checkRaid 检查 RAID 状态
func (s *AlertService) checkRaid() {
	// MD RAID
	if data, err := os.ReadFile("/proc/mdstat"); err == nil {
		content := string(data)
		if strings.Contains(content, "_") && !strings.Contains(content, "unused") {
			s.addAlert(Alert{
				ID: fmt.Sprintf("raid_md_%d", time.Now().Unix()),
				Type: AlertRaidDegraded, Level: AlertLevelCritical,
				Title: "RAID 阵列降级", Message: "MD RAID 阵列检测到降级状态",
				Source: "mdraid", Timestamp: time.Now(),
			})
		}
	}

	// ZFS
	if output, err := exec.Command("zpool", "status", "-x").Output(); err == nil {
		result := strings.TrimSpace(string(output))
		if result != "all pools are healthy" {
			s.addAlert(Alert{
				ID: fmt.Sprintf("raid_zfs_%d", time.Now().Unix()),
				Type: AlertRaidDegraded, Level: AlertLevelCritical,
				Title: "ZFS 池异常", Message: "ZFS 存储池检测到异常状态",
				Source: "zfs", Timestamp: time.Now(),
			})
		}
	}
}
