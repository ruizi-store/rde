package bot

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/ruizi-store/rde/backend/modules/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// fakeDocker 模拟 docker 能力（不依赖真实 daemon）
type fakeDocker struct {
	running     bool
	created     []*docker.ContainerConfig
	containerID string
	status      *docker.ContainerStatus
	execResult  *docker.ExecResult
	execCmd     []string
	ip          string
}

func (f *fakeDocker) IsRunning(ctx context.Context) bool { return f.running }
func (f *fakeDocker) CreateContainer(ctx context.Context, cfg *docker.ContainerConfig) (string, error) {
	f.created = append(f.created, cfg)
	f.containerID = "cid-123"
	return "cid-123", nil
}
func (f *fakeDocker) StartContainer(ctx context.Context, id string) error { return nil }
func (f *fakeDocker) StopContainer(ctx context.Context, id string) error  { return nil }
func (f *fakeDocker) RemoveContainer(ctx context.Context, id string, force bool) error {
	f.containerID = ""
	return nil
}
func (f *fakeDocker) GetContainerStatus(ctx context.Context, id string) (*docker.ContainerStatus, error) {
	if f.status != nil {
		return f.status, nil
	}
	return &docker.ContainerStatus{ID: id, Running: true, State: "running"}, nil
}
func (f *fakeDocker) GetContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	return "fake logs", nil
}
func (f *fakeDocker) ExecWithTimeout(ctx context.Context, id string, cmd []string, timeoutSec, maxOutput int) (*docker.ExecResult, error) {
	f.execCmd = cmd
	if f.execResult != nil {
		return f.execResult, nil
	}
	return &docker.ExecResult{ExitCode: 0, Output: "ok"}, nil
}
func (f *fakeDocker) GetContainerNetworkIP(ctx context.Context, id, network string) (string, error) {
	return f.ip, nil
}
func (f *fakeDocker) ListNetworks(ctx context.Context) ([]docker.NetworkInfo, error) {
	return nil, nil
}
func (f *fakeDocker) CreateNetwork(ctx context.Context, name, driver string) (string, error) {
	return "netid", nil
}

func setupService(t *testing.T) (*Service, *fakeDocker) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Bot{}, &BotMember{}))

	fd := &fakeDocker{running: true, ip: "172.20.0.2"}
	svc := NewService(db, zap.NewNop(), t.TempDir(), fd)
	return svc, fd
}

func TestService_CRUD(t *testing.T) {
	svc, _ := setupService(t)

	b, err := svc.Create("小明", "日报助手", "qwen3", "p1", "")
	require.NoError(t, err)
	assert.Equal(t, StatusCreated, b.Status)
	assert.Equal(t, DefaultImage, b.Image)

	// 重名拒绝
	_, err = svc.Create("小明", "", "", "", "")
	assert.Error(t, err)

	// List / Get
	list, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 1)

	got, err := svc.Get(context.Background(), b.ID)
	require.NoError(t, err)
	assert.Equal(t, "小明", got.Name)

	// Update
	newName := "小美"
	got, err = svc.Update(b.ID, &newName, nil, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "小美", got.Name)

	// Delete
	require.NoError(t, svc.Delete(context.Background(), b.ID, true))
	_, err = svc.Get(context.Background(), b.ID)
	assert.Error(t, err)
}

func TestService_StartSandbox(t *testing.T) {
	svc, fd := setupService(t)
	ctx := context.Background()

	b, err := svc.Create("小明", "", "", "", "")
	require.NoError(t, err)

	b, err = svc.StartSandbox(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusRunning, b.Status)
	assert.Equal(t, "cid-123", b.ContainerID)

	// 验证容器创建参数：网络/卷/label/重启策略
	require.Len(t, fd.created, 1)
	cfg := fd.created[0]
	assert.Equal(t, containerName(b.ID), cfg.Name)
	assert.Equal(t, []string{NetworkName}, cfg.Networks)
	assert.Len(t, cfg.Volumes, 1)
	assert.Contains(t, cfg.Volumes[svc.workspaceDir(b.ID)], WorkspacePath)
	assert.Equal(t, b.ID, cfg.Labels[ManagedLabel])
	assert.Equal(t, "unless-stopped", cfg.Restart)

	// 串流地址
	addr, err := svc.GetSandboxAddr(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "http://172.20.0.2:6901", addr)

	// 二次启动幂等（不重复创建容器）
	_, err = svc.StartSandbox(ctx, b.ID)
	require.NoError(t, err)
	assert.Len(t, fd.created, 1)

	// 停止
	b, err = svc.StopSandbox(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusStopped, b.Status)
}

func TestService_MembersAndRoles(t *testing.T) {
	svc, _ := setupService(t)

	b, _ := svc.Create("小明", "", "", "", "")
	require.NoError(t, svc.SetMembers(b.ID, []BotMember{
		{UserID: "u1", Role: RoleMemberAdmin},
		{UserID: "u2", Role: RoleMemberMember},
		{UserID: "u3", Role: RoleMemberViewer},
	}))

	members, err := svc.ListMembers(b.ID)
	require.NoError(t, err)
	assert.Len(t, members, 3)

	assert.Equal(t, RoleMemberAdmin, svc.GetRole("u1", "user", b.ID))
	assert.Equal(t, RoleMemberViewer, svc.GetRole("u3", "user", b.ID))
	// 非成员不可见
	assert.Equal(t, "", svc.GetRole("u4", "user", b.ID))
	// 平台管理员 = bot admin
	assert.Equal(t, RoleMemberAdmin, svc.GetRole("u4", "admin", b.ID))

	// 无效角色拒绝
	err = svc.SetMembers(b.ID, []BotMember{{UserID: "u1", Role: "owner"}})
	assert.Error(t, err)
}

func TestService_TakeoverLock(t *testing.T) {
	svc, _ := setupService(t)

	b, _ := svc.Create("小明", "", "", "", "")

	// u1 接管成功
	require.NoError(t, svc.Takeover(b.ID, "u1", "alice"))
	// u2 接管被拒（冲突）
	err := svc.Takeover(b.ID, "u2", "bob")
	assert.Error(t, err)
	// u1 重复接管幂等
	require.NoError(t, svc.Takeover(b.ID, "u1", "alice"))
	// u1 释放；u2 接管成功
	require.NoError(t, svc.Release(b.ID, "u1"))
	require.NoError(t, svc.Takeover(b.ID, "u2", "bob"))
	// 非 takeover 持有者不能释放
	assert.Error(t, svc.Release(b.ID, "u1"))
}

func TestService_Exec(t *testing.T) {
	svc, fd := setupService(t)
	ctx := context.Background()

	b, _ := svc.Create("小明", "", "", "", "")
	_, err := svc.Exec(ctx, b.ID, &ExecRequest{Script: "ls", Language: "bash"})
	assert.ErrorContains(t, err, "沙箱未运行")

	// 启动后执行
	_, err = svc.StartSandbox(ctx, b.ID)
	require.NoError(t, err)
	res, err := svc.Exec(ctx, b.ID, &ExecRequest{Script: "ls /workspace", Language: "bash", Timeout: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode)
	// 脚本作为解释器 -c 参数传入（不拼进 shell 命令行）
	require.Equal(t, []string{"bash", "-c", "ls /workspace"}, fd.execCmd)

	// 不支持的语言拒绝
	_, err = svc.Exec(ctx, b.ID, &ExecRequest{Script: "x", Language: "node"})
	assert.Error(t, err)
}
