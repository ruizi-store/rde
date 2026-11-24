package bot

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ruizi-store/rde/backend/modules/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestParseSkillFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))

	// 无 frontmatter：name 取目录名
	path := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(path, []byte("做点事。"), 0644))
	skill, err := parseSkillFile(path, skillDir)
	require.NoError(t, err)
	assert.Equal(t, "my-skill", skill.Name)
	assert.Equal(t, "做点事。", skill.Content)

	// 有 frontmatter
	require.NoError(t, os.WriteFile(path, []byte("---\nname: web-research\ndescription: 网页检索\n---\n\n# 正文\n\n步骤..."), 0644))
	skill, err = parseSkillFile(path, skillDir)
	require.NoError(t, err)
	assert.Equal(t, "web-research", skill.Name)
	assert.Equal(t, "网页检索", skill.Description)
	assert.Equal(t, "# 正文\n\n步骤...", skill.Content)

	// 正文为空
	require.NoError(t, os.WriteFile(path, []byte("---\nname: x\n---\n"), 0644))
	_, err = parseSkillFile(path, skillDir)
	assert.Error(t, err)
}

func TestSkillService_SeedAndList(t *testing.T) {
	dir := t.TempDir()
	svc := NewSkillService(zap.NewNop(), dir)

	// 首次创建播种默认技能
	skills := svc.ListSkills()
	require.NotEmpty(t, skills)
	names := map[string]bool{}
	for _, s := range skills {
		names[s.Name] = true
		assert.NotEmpty(t, s.Description)
	}
	assert.True(t, names["web-research"])
	assert.True(t, names["daily-report"])

	// 默认技能含正文与入口脚本
	content, err := svc.GetSkillContent("web-research")
	require.NoError(t, err)
	assert.Contains(t, content, "网页研究")

	_, err = os.Stat(filepath.Join(dir, "skills", "daily-report", "scripts", "run.sh"))
	require.NoError(t, err)

	// 不存在的技能
	_, err = svc.GetSkillContent("nope")
	assert.Error(t, err)

	// 二次创建不重复播种（目录已存在）
	svc2 := NewSkillService(zap.NewNop(), dir)
	assert.Equal(t, len(skills), len(svc2.ListSkills()))

	// Reload 热加载：手动加一个技能后可见
	extra := filepath.Join(dir, "skills", "extra")
	require.NoError(t, os.MkdirAll(extra, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(extra, "SKILL.md"), []byte("---\nname: extra\ndescription: 附加\n---\n内容"), 0644))
	require.NoError(t, svc.Reload())
	_, err = svc.GetSkillContent("extra")
	require.NoError(t, err)
}

func TestBotSandbox_RunSkill_ArgInjection(t *testing.T) {
	svc, fd := setupService(t)
	skills := NewSkillService(zap.NewNop(), t.TempDir())

	b, err := svc.Create("跑技能", "", "", "", "")
	require.NoError(t, err)
	// 直接置为运行态，绕过真实容器创建（fakeDocker）
	require.NoError(t, svc.db.Model(&Bot{}).Where("id = ?", b.ID).
		Updates(map[string]interface{}{"status": StatusRunning, "container_id": "cid-123"}).Error)

	// 手工放置技能脚本
	skillDir := filepath.Join(skills.skillsDir, "hello")
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "scripts"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: hello\ndescription: 问好\n---\n问好"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "scripts", "run.sh"),
		[]byte("#!/bin/bash\necho \"args=$SKILL_ARGS\"\n"), 0755))
	require.NoError(t, skills.Reload())

	sb := &botSandbox{service: svc, skills: skills, logger: zap.NewNop()}
	out, err := sb.RunSkill(context.Background(), b.ID, "hello", `{"title":"周报"}`)
	require.NoError(t, err)
	assert.Equal(t, "ok", out) // fakeDocker 固定返回

	// 脚本前缀应包含 base64 解码注入，且脚本体完整
	script := fd.execCmd[2]
	assert.Contains(t, script, "base64 -d")
	assert.Contains(t, script, "#!/bin/bash")

	// 非零退出码返回错误
	fd.execResult = &docker.ExecResult{ExitCode: 1, Output: "boom"}
	_, err = sb.RunSkill(context.Background(), b.ID, "hello", "")
	require.Error(t, err)
}

func TestB64Arg(t *testing.T) {
	assert.Equal(t, "e30=", b64Arg(""))
	assert.Equal(t, "e30=", b64Arg("  "))
	assert.Contains(t, pythonB64Arg(`{"a":1}`), "base64.b64decode")
}
