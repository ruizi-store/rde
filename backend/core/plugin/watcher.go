package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// StartWatching 启动 fsnotify 监听插件目录
// 在插件目录下出现新子目录时自动加载并启动，子目录删除时自动停止并卸载
func (m *Manager) StartWatching() {
	// 确保插件目录存在
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		m.logger.Error("Failed to create plugin directory for watching",
			zap.String("dir", m.pluginDir),
			zap.Error(err))
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		m.logger.Error("Failed to create fsnotify watcher", zap.Error(err))
		return
	}

	go m.watchLoop(watcher)

	// 监听插件根目录
	if err := watcher.Add(m.pluginDir); err != nil {
		m.logger.Error("Failed to watch plugin directory",
			zap.String("dir", m.pluginDir),
			zap.Error(err))
		watcher.Close()
		return
	}

	// 同时监听已有插件子目录（检测 manifest.json / binary 变化触发重载）
	m.mu.RLock()
	for _, p := range m.plugins {
		if err := watcher.Add(p.dir); err != nil {
			m.logger.Warn("Failed to watch plugin directory",
				zap.String("plugin", p.manifest.ID),
				zap.Error(err))
		}
	}
	m.mu.RUnlock()

	m.logger.Info("Plugin directory watcher started",
		zap.String("dir", m.pluginDir))
}

// watchLoop 主监听循环
func (m *Manager) watchLoop(watcher *fsnotify.Watcher) {
	defer watcher.Close()

	for {
		select {
		case <-m.stopCh:
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			m.handleFSEvent(watcher, event)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("Watcher error", zap.Error(err))
		}
	}
}

// handleFSEvent 处理文件系统事件
func (m *Manager) handleFSEvent(watcher *fsnotify.Watcher, event fsnotify.Event) {
	// 只关注插件根目录下的直接子目录变化
	rel, err := filepath.Rel(m.pluginDir, event.Name)
	if err != nil {
		return
	}

	// 忽略深层嵌套的文件事件（只处理直接子目录或子目录内的关键文件）
	dir := filepath.Dir(rel)
	baseName := filepath.Base(rel)

	if dir == "." {
		// 直接子条目：新建或删除了一个插件目录
		m.handlePluginDirEvent(watcher, event)
	} else if filepath.Dir(dir) == "." && (baseName == "manifest.json" || baseName == "plugin") {
		// 插件目录内的关键文件变化：触发重载
		pluginDirName := dir
		if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
			m.logger.Info("Plugin file changed, scheduling reload",
				zap.String("dir", pluginDirName),
				zap.String("file", baseName))
			go m.delayedReload(filepath.Join(m.pluginDir, pluginDirName))
		}
	}
}

// handlePluginDirEvent 处理插件目录的创建/删除
func (m *Manager) handlePluginDirEvent(watcher *fsnotify.Watcher, event fsnotify.Event) {
	fullPath := event.Name

	if event.Has(fsnotify.Create) {
		// 新目录出现：检查是否是有效插件目录
		info, err := os.Stat(fullPath)
		if err != nil || !info.IsDir() {
			return
		}

		// 监听新插件目录内的文件变化
		watcher.Add(fullPath)

		m.logger.Info("New plugin directory detected",
			zap.String("path", fullPath))

		// 延迟加载，等待文件写入完成（如解压操作）
		go m.delayedLoad(fullPath)
	}

	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		m.logger.Info("Plugin directory removed",
			zap.String("path", fullPath))
		m.hotUnloadByDir(fullPath)
		watcher.Remove(fullPath) // 忽略错误，目录可能已不存在
	}
}

// delayedLoad 延迟加载插件（等待文件写入完成）
func (m *Manager) delayedLoad(dir string) {
	select {
	case <-m.stopCh:
		return
	case <-time.After(2 * time.Second):
	}
	m.hotLoadPlugin(dir)
}

// delayedReload 延迟重载插件
func (m *Manager) delayedReload(dir string) {
	select {
	case <-m.stopCh:
		return
	case <-time.After(2 * time.Second):
	}
	m.reloadPlugin(dir)
}

// hotLoadPlugin 热加载一个新插件：读取 manifest → 启动进程
func (m *Manager) hotLoadPlugin(dir string) {
	// 检查 manifest.json 是否存在
	manifestPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		m.logger.Debug("No manifest.json found, skipping",
			zap.String("dir", dir))
		return
	}

	// 加载插件（loadPlugin 内部会检查重复）
	if err := m.loadPlugin(dir); err != nil {
		m.logger.Warn("Hot-load plugin failed",
			zap.String("dir", dir),
			zap.Error(err))
		return
	}

	// 找到刚加载的插件并启动
	m.mu.Lock()
	var loaded *pluginInstance
	for _, p := range m.plugins {
		if p.dir == dir {
			loaded = p
			break
		}
	}

	if loaded == nil {
		m.mu.Unlock()
		return
	}

	// 检查插件是否被禁用
	if m.isDisabled(loaded.manifest.ID) {
		m.logger.Info("Plugin hot-loaded but disabled, not starting",
			zap.String("id", loaded.manifest.ID))
		m.mu.Unlock()
		return
	}

	if err := m.startPlugin(loaded); err != nil {
		m.logger.Error("Hot-start plugin failed",
			zap.String("id", loaded.manifest.ID),
			zap.Error(err))
		loaded.state = StateError
		loaded.errMsg = err.Error()
	}
	m.mu.Unlock()

	if loaded != nil {
		m.logger.Info("Plugin hot-loaded successfully",
			zap.String("id", loaded.manifest.ID),
			zap.String("dir", dir))
	}
}

// hotUnloadByDir 根据目录路径卸载插件
func (m *Manager) hotUnloadByDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var targetID string
	for id, p := range m.plugins {
		if p.dir == dir {
			targetID = id
			break
		}
	}

	if targetID == "" {
		return
	}

	m.stopPluginLocked(targetID)
	delete(m.plugins, targetID)

	m.logger.Info("Plugin hot-unloaded",
		zap.String("id", targetID),
		zap.String("dir", dir))
}

// reloadPlugin 重载插件：停止 → 卸载 → 重新加载 → 启动
func (m *Manager) reloadPlugin(dir string) {
	m.mu.Lock()

	// 找到现有实例
	var oldID string
	for id, p := range m.plugins {
		if p.dir == dir {
			oldID = id
			break
		}
	}

	// 如果已有实例，先停止并删除
	if oldID != "" {
		m.stopPluginLocked(oldID)
		delete(m.plugins, oldID)
		m.logger.Info("Old plugin instance removed for reload",
			zap.String("id", oldID))
	}
	m.mu.Unlock()

	// 重新加载
	m.hotLoadPlugin(dir)
}

// stopPluginLocked 停止指定插件（调用者须持有 mu 写锁）
func (m *Manager) stopPluginLocked(id string) {
	p, ok := m.plugins[id]
	if !ok {
		return
	}

	if p.cmd == nil || p.cmd.Process == nil {
		p.state = StateStopped
		return
	}
	if p.state != StateRunning && p.state != StateStarting {
		return
	}

	p.state = StateStopped

	// SIGTERM 优雅关闭
	p.cmd.Process.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func(cmd *exec.Cmd) {
		cmd.Wait()
		close(done)
	}(p.cmd)

	select {
	case <-done:
		m.logger.Info("Plugin stopped gracefully", zap.String("id", id))
	case <-time.After(5 * time.Second):
		p.cmd.Process.Kill()
		m.logger.Warn("Plugin force killed", zap.String("id", id))
	}

	// 清理 socket
	os.Remove(p.socketPath)
}
