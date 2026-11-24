// Package bot — 技能文件化（SKILL.md）
//
// 技能目录结构（对齐 agent skills 生态约定）：
//
//	/var/lib/rde/skills/<name>/
//	  SKILL.md          # frontmatter(name/description) + 正文指令（给 LLM）
//	  scripts/run.sh    # 可选：预置入口脚本（run_skill 执行）
//	  scripts/run.py    # 可选：Python 入口，与 run.sh 二选一
//
// 渐进式披露：网关对话只注入 name+description 索引；LLM 按需 get_skill 读全文，
// 再生成脚本经 execute_script 在 Bot 沙箱执行，或 run_skill 直接跑预置脚本。
package bot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ruizi-store/rde/backend/modules/ai"
	"go.uber.org/zap"
)

// SkillFile 解析后的技能文件
type SkillFile struct {
	Name        string // frontmatter name，缺省取目录名
	Description string // frontmatter description
	Dir         string // 技能目录绝对路径
	Content     string // SKILL.md 正文（frontmatter 之后）
}

// SkillService 技能文件服务
type SkillService struct {
	logger    *zap.Logger
	skillsDir string
	mu        sync.RWMutex
	skills    map[string]*SkillFile // key: skill name
}

// NewSkillService 创建技能服务并扫描目录
func NewSkillService(logger *zap.Logger, dataDir string) *SkillService {
	svc := &SkillService{
		logger:    logger,
		skillsDir: filepath.Join(dataDir, "skills"),
		skills:    make(map[string]*SkillFile),
	}
	// 播种须在 MkdirAll 之前判断目录是否已存在（首次运行才播种）
	svc.seedDefaultSkills()
	if err := os.MkdirAll(svc.skillsDir, 0755); err != nil {
		logger.Warn("failed to create skills dir", zap.String("dir", svc.skillsDir), zap.Error(err))
	}
	svc.Reload()
	return svc
}

// Reload 重新扫描技能目录（热加载，REST 触发）
func (s *SkillService) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.skills = make(map[string]*SkillFile)
	entries, err := os.ReadDir(s.skillsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skill, err := parseSkillFile(filepath.Join(s.skillsDir, e.Name(), "SKILL.md"), filepath.Join(s.skillsDir, e.Name()))
		if err != nil {
			s.logger.Warn("skip invalid skill", zap.String("dir", e.Name()), zap.Error(err))
			continue
		}
		s.skills[skill.Name] = skill
	}
	s.logger.Info("skills loaded", zap.Int("count", len(s.skills)))
	return nil
}

// ListSkills 技能索引（name + description）
func (s *SkillService) ListSkills() []ai.SkillMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ai.SkillMeta, 0, len(s.skills))
	for _, sk := range s.skills {
		result = append(result, ai.SkillMeta{Name: sk.Name, Description: sk.Description})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// GetSkillContent 读取技能正文指令
func (s *SkillService) GetSkillContent(name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sk, ok := s.skills[name]
	if !ok {
		return "", fmt.Errorf("技能 '%s' 不存在", name)
	}
	return sk.Content, nil
}

// GetSkill 返回技能文件（含目录，供 run_skill 定位脚本）
func (s *SkillService) GetSkill(name string) (*SkillFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sk, ok := s.skills[name]
	if !ok {
		return nil, fmt.Errorf("技能 '%s' 不存在", name)
	}
	return sk, nil
}

// parseSkillFile 解析 SKILL.md：YAML frontmatter 子集（name/description）+ 正文
func parseSkillFile(path, dir string) (*SkillFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 SKILL.md 失败: %w", err)
	}

	skill := &SkillFile{Dir: dir}
	content := string(data)

	// frontmatter：以 --- 开始并以 --- 结束的首个块
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end >= 0 {
			frontmatter := content[4 : end+4]
			content = strings.TrimLeft(content[end+4+4:], "\n")
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if v, ok := strings.CutPrefix(line, "name:"); ok && skill.Name == "" {
					skill.Name = strings.TrimSpace(v)
				} else if v, ok := strings.CutPrefix(line, "description:"); ok {
					skill.Description = strings.TrimSpace(v)
				}
			}
		}
	}
	// frontmatter 未提供 name 时回退目录名
	if skill.Name == "" {
		skill.Name = filepath.Base(dir)
	}
	skill.Content = strings.TrimSpace(content)
	if skill.Content == "" {
		return nil, fmt.Errorf("SKILL.md 正文为空")
	}
	return skill, nil
}

// seedDefaultSkills 首次运行播种默认技能（目录已存在则跳过，绝不覆盖用户文件）
func (s *SkillService) seedDefaultSkills() {
	if _, err := os.Stat(s.skillsDir); err == nil {
		return
	}
	if err := os.MkdirAll(s.skillsDir, 0755); err != nil {
		return
	}

	defaults := map[string]struct {
		skill  string
		script string
	}{
		"web-research": {
			skill: `---
name: web-research
description: 在沙箱浏览器中检索网页信息并整理要点（Chromium 已预装）
---

# 网页研究

你是数字员工的网页研究员。任务：围绕用户给的主题收集信息并归纳。

## 步骤

1. 用 execute_script 在沙箱中执行 python 脚本，通过 requests 或 curl 抓取目标页面
   （沙箱可以访问外网时优先 curl；失败则提示用户改用人工浏览）。
2. 提取正文关键信息，过滤导航/广告噪音。
3. 以中文要点列表输出结论，附来源 URL。

## 注意

- 每次抓取设置超时（≤30s）与 UA。
- 页面较多时先抓 3 个最相关来源，够用即止。
- 无法访问时如实说明，不要编造内容。
`,
		},
		"daily-report": {
			skill: `---
name: daily-report
description: 汇总沙箱工作产出，生成结构化日报（Markdown）
---

# 日报生成

## 说明

本技能附带预置入口脚本 scripts/run.sh，可用 run_skill 直接执行，
脚本会把日报骨架写到 /root/daily-report.md。

## 步骤（LLM 自主模式）

1. execute_script 执行 bash：ls /root 收集当天产出文件。
2. 按模板生成 Markdown 日报（日期/完成事项/待办/风险）。
3. 写入 /root/daily-report.md 并在回复中给出要点。
`,
			script: `#!/bin/bash
# daily-report 预置入口：生成日报骨架（参数从 SKILL_ARGS 读取 JSON）
set -euo pipefail
TITLE=$(printf '%s' "${SKILL_ARGS:-{\}}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("title", "工作日报"))' 2>/dev/null || echo "工作日报")
DATE=$(date +%F)
cat > /root/daily-report.md <<EOF
# ${TITLE} - ${DATE}

## 今日完成
- （待填写）

## 待办
- （待填写）

## 风险与求助
- （无）
EOF
echo "日报骨架已写入 /root/daily-report.md"
`,
		},
	}

	for name, d := range defaults {
		dir := filepath.Join(s.skillsDir, name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(d.skill), 0644); err != nil {
			s.logger.Warn("failed to seed skill", zap.String("name", name), zap.Error(err))
			continue
		}
		if d.script != "" {
			scriptDir := filepath.Join(dir, "scripts")
			if err := os.MkdirAll(scriptDir, 0755); err == nil {
				_ = os.WriteFile(filepath.Join(scriptDir, "run.sh"), []byte(d.script), 0755)
			}
		}
	}
	s.logger.Info("default skills seeded", zap.Int("count", len(defaults)))
}
