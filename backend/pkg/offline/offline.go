// Package offline 提供本地离线资源（Docker 镜像 tar、deb、驱动）优先加载能力。
//
// 目录约定（可由配置 offline.dir 覆盖，默认 /usr/share/rde/offline）：
//
//	{dir}/manifest.yaml
//	{dir}/images/*.tar[.gz]
//	{dir}/debs/*.deb
//	{dir}/drivers/*
package offline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const DefaultDir = "/usr/share/rde/offline"

// Manifest 描述可离线安装的资源
type Manifest struct {
	Version string            `yaml:"version"`
	Images  map[string]string `yaml:"images"`  // 逻辑名或镜像引用 -> 相对路径
	Debs    map[string]string `yaml:"debs"`    // 包名 -> 相对路径
	Drivers map[string]string `yaml:"drivers"` // 驱动名 -> 相对路径
}

// Store 离线资源仓库
type Store struct {
	dir      string
	manifest *Manifest
	mu       sync.RWMutex
}

var (
	globalStore *Store
	globalOnce  sync.Once
)

// InitGlobal 初始化全局离线仓库（可重复调用以刷新配置）
func InitGlobal(dir string) *Store {
	if dir == "" {
		dir = DefaultDir
	}
	globalOnce.Do(func() {})
	globalStore = New(dir)
	return globalStore
}

// Global 返回全局仓库（未初始化时使用默认目录）
func Global() *Store {
	if globalStore == nil {
		return InitGlobal(DefaultDir)
	}
	return globalStore
}

// New 创建离线仓库并尝试加载 manifest
func New(dir string) *Store {
	s := &Store{dir: dir}
	s.Reload()
	return s
}

// Dir 返回根目录
func (s *Store) Dir() string {
	if s == nil || s.dir == "" {
		return DefaultDir
	}
	return s.dir
}

// Reload 重新加载 manifest.yaml
func (s *Store) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.Dir(), "manifest.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		s.manifest = &Manifest{
			Version: "1",
			Images:  map[string]string{},
			Debs:    map[string]string{},
			Drivers: map[string]string{},
		}
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parse offline manifest: %w", err)
	}
	if m.Images == nil {
		m.Images = map[string]string{}
	}
	if m.Debs == nil {
		m.Debs = map[string]string{}
	}
	if m.Drivers == nil {
		m.Drivers = map[string]string{}
	}
	s.manifest = &m
	return nil
}

func (s *Store) resolve(rel string) string {
	if rel == "" {
		return ""
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(s.Dir(), rel)
}

// FindImageTar 按镜像引用或逻辑名查找本地 tar
func (s *Store) FindImageTar(imageRef string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manifest == nil {
		return "", false
	}

	candidates := []string{imageRef}
	// 兼容去掉 registry 前缀 / :tag
	if i := strings.LastIndex(imageRef, "/"); i >= 0 {
		candidates = append(candidates, imageRef[i+1:])
	}
	if i := strings.Index(imageRef, ":"); i >= 0 {
		candidates = append(candidates, imageRef[:i])
		base := imageRef
		if j := strings.LastIndex(imageRef, "/"); j >= 0 {
			base = imageRef[j+1:]
		}
		if k := strings.Index(base, ":"); k >= 0 {
			candidates = append(candidates, base[:k])
		}
	}

	for _, key := range candidates {
		if rel, ok := s.manifest.Images[key]; ok {
			p := s.resolve(rel)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}

	// 约定路径回退：images/<sanitized>.tar
	sanitized := strings.NewReplacer("/", "_", ":", "_").Replace(imageRef)
	for _, name := range []string{sanitized + ".tar", sanitized + ".tar.gz", sanitized + ".tgz"} {
		p := filepath.Join(s.Dir(), "images", name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

// HasImage 本地是否存在对应镜像 tar（尚未 docker load 也算有）
func (s *Store) HasImage(imageRef string) bool {
	_, ok := s.FindImageTar(imageRef)
	return ok
}

// LoadImage 若本地存在镜像 tar，则 docker load；成功返回 true
func (s *Store) LoadImage(imageRef string) (bool, error) {
	tarPath, ok := s.FindImageTar(imageRef)
	if !ok {
		return false, nil
	}

	cmd := exec.Command("docker", "load", "-i", tarPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("docker load %s: %w (%s)", tarPath, err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

// FindDeb 查找本地 deb
func (s *Store) FindDeb(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manifest != nil {
		if rel, ok := s.manifest.Debs[name]; ok {
			p := s.resolve(rel)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}
	// 约定回退
	for _, pattern := range []string{
		filepath.Join(s.Dir(), "debs", name+".deb"),
		filepath.Join(s.Dir(), "debs", name+"_*_amd64.deb"),
		filepath.Join(s.Dir(), "debs", name+"_*_arm64.deb"),
	} {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return matches[0], true
		}
	}
	return "", false
}

// FindDriver 查找本地驱动/模块包路径
func (s *Store) FindDriver(name string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.manifest != nil {
		if rel, ok := s.manifest.Drivers[name]; ok {
			p := s.resolve(rel)
			if _, err := os.Stat(p); err == nil {
				return p, true
			}
		}
	}
	p := filepath.Join(s.Dir(), "drivers", name)
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}
