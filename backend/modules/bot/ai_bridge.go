// Package bot — ai 模块桥接实现（实现 ai.SandboxExecutor / ai.BotResolver）
//
// 依赖方向：bot → ai（ai 不反向依赖 bot，见 ai/bot_bridge.go）。
// 工具执行全部走 Bot 沙箱容器（bot.Service.Exec → docker ExecWithTimeout），
// 控制面进程不执行任何用户/LLM 生成的脚本。
package bot

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ruizi-store/rde/backend/modules/ai"
	"go.uber.org/zap"
)

// botSandbox 实现 ai.SandboxExecutor
type botSandbox struct {
	service *Service
	skills  *SkillService
	logger  *zap.Logger
}

// ExecInBot 在 Bot 沙箱内执行脚本
func (b *botSandbox) ExecInBot(ctx context.Context, botID, language, script string, timeoutSec int) (*ai.BotExecResult, error) {
	result, err := b.service.Exec(ctx, botID, &ExecRequest{
		Script:   script,
		Language: language,
		Timeout:  timeoutSec,
	})
	if err != nil {
		return nil, err
	}
	return &ai.BotExecResult{
		ExitCode:  result.ExitCode,
		Output:    result.Output,
		Truncated: result.Truncated,
	}, nil
}

// RunSkill 执行技能预置入口脚本（scripts/run.sh 或 run.py）
// 参数以 base64 编码注入 SKILL_ARGS 环境变量，避免拼接命令行注入。
func (b *botSandbox) RunSkill(ctx context.Context, botID, skillName, argsJSON string) (string, error) {
	skill, err := b.skills.GetSkill(skillName)
	if err != nil {
		return "", err
	}

	// 按优先级找入口脚本：run.sh → run.py
	var script, language, prefix string
	if p := filepath.Join(skill.Dir, "scripts", "run.sh"); fileExists(p) {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("读取技能脚本失败: %w", err)
		}
		script, language = string(data), "bash"
		prefix = fmt.Sprintf("SKILL_ARGS=$(printf %%s %s | base64 -d)\n", b64Arg(argsJSON))
	} else if p := filepath.Join(skill.Dir, "scripts", "run.py"); fileExists(p) {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("读取技能脚本失败: %w", err)
		}
		script, language = string(data), "python"
		prefix = fmt.Sprintf("import os,base64\nos.environ[\"SKILL_ARGS\"]=%s\n", pythonB64Arg(argsJSON))
	} else {
		return "", fmt.Errorf("技能 '%s' 无预置入口脚本（scripts/run.sh 或 run.py）", skillName)
	}

	result, err := b.service.Exec(ctx, botID, &ExecRequest{
		Script:   prefix + script,
		Language: language,
		Timeout:  120,
	})
	if err != nil {
		return "", err
	}
	output := strings.TrimSpace(result.Output)
	if result.ExitCode != 0 {
		return output, fmt.Errorf("技能脚本退出码 %d", result.ExitCode)
	}
	if result.Truncated {
		output += "\n（输出超限已截断）"
	}
	return output, nil
}

// b64Arg 参数 JSON 的 base64（shell 安全注入）
func b64Arg(argsJSON string) string {
	if strings.TrimSpace(argsJSON) == "" {
		argsJSON = "{}"
	}
	return base64.StdEncoding.EncodeToString([]byte(argsJSON))
}

// pythonB64Arg 参数 JSON 的 base64（Python 字面量形式）
func pythonB64Arg(argsJSON string) string {
	return fmt.Sprintf("base64.b64decode(%q).decode()", b64Arg(argsJSON))
}

// fileExists 判断文件存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// botChannelResolver 实现 ai.BotResolver
type botChannelResolver struct {
	service *Service
}

// ResolveBotByChannel 按渠道解析 Bot，转换为 ai.BotInfo（未命中返回 nil）
func (r *botChannelResolver) ResolveBotByChannel(platform, chatID, userID string) *ai.BotInfo {
	b := r.service.ResolveBotByChannel(platform, chatID, userID)
	if b == nil {
		return nil
	}
	return &ai.BotInfo{
		ID:          b.ID,
		Name:        b.Name,
		Description: b.Description,
		Model:       b.Model,
		ProviderID:  b.ProviderID,
	}
}
