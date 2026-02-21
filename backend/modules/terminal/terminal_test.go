// Package terminal 终端模块 - 单元测试
package terminal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewSessionManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	defer sm.Stop()

	assert.NotNil(t, sm)
	assert.NotNil(t, sm.sessions)
	assert.NotNil(t, sm.userIndex)
}

func TestCreateSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	defer sm.Stop()

	req := &CreateSessionRequest{
		Cols:  80,
		Rows:  24,
		Shell: "/bin/sh", // 使用更通用的 shell
	}

	session, err := sm.CreateSession(1, "testuser", req)
	require.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, uint(1), session.UserID)
	assert.Equal(t, "testuser", session.Username)
	assert.Equal(t, uint16(80), session.Cols)
	assert.Equal(t, uint16(24), session.Rows)
	assert.NotEmpty(t, session.ID)
	assert.True(t, session.ID[:5] == "term_")

	// 验证会话已注册
	s, ok := sm.GetSession(session.ID)
	assert.True(t, ok)
	assert.Equal(t, session.ID, s.ID)

	// 清理
	err = sm.CloseSession(session.ID)
	assert.NoError(t, err)
}

func TestCreateSessionDefaults(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	defer sm.Stop()

	// 使用空请求，应该使用默认值
	req := &CreateSessionRequest{}

	session, err := sm.CreateSession(1, "testuser", req)
	require.NoError(t, err)
	assert.Equal(t, uint16(DefaultCols), session.Cols)
	assert.Equal(t, uint16(DefaultRows), session.Rows)

	// 清理
	sm.CloseSession(session.ID)
}

func TestMaxSessionsPerUser(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	sm.maxSessionsPerUser = 2 // 设置较小的限制便于测试
	defer sm.Stop()

	req := &CreateSessionRequest{Shell: "/bin/sh"}

	// 创建第一个会话
	s1, err := sm.CreateSession(1, "testuser", req)
	require.NoError(t, err)

	// 创建第二个会话
	s2, err := sm.CreateSession(1, "testuser", req)
	require.NoError(t, err)

	// 创建第三个会话应该失败
	_, err = sm.CreateSession(1, "testuser", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeded maximum sessions")

	// 清理
	sm.CloseSession(s1.ID)
	sm.CloseSession(s2.ID)
}

func TestGetUserSessions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	defer sm.Stop()

	req := &CreateSessionRequest{Shell: "/bin/sh"}

	// 用户1创建2个会话
	s1, _ := sm.CreateSession(1, "user1", req)
	s2, _ := sm.CreateSession(1, "user1", req)

	// 用户2创建1个会话
	s3, _ := sm.CreateSession(2, "user2", req)

	// 获取用户1的会话
	user1Sessions := sm.GetUserSessions(1)
	assert.Len(t, user1Sessions, 2)

	// 获取用户2的会话
	user2Sessions := sm.GetUserSessions(2)
	assert.Len(t, user2Sessions, 1)

	// 获取所有会话
	allSessions := sm.GetAllSessions()
	assert.Len(t, allSessions, 3)

	// 清理
	sm.CloseSession(s1.ID)
	sm.CloseSession(s2.ID)
	sm.CloseSession(s3.ID)
}

func TestResize(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	defer sm.Stop()

	req := &CreateSessionRequest{
		Cols:  80,
		Rows:  24,
		Shell: "/bin/sh",
	}

	session, err := sm.CreateSession(1, "testuser", req)
	require.NoError(t, err)

	// 调整大小
	err = sm.Resize(session.ID, 120, 40)
	assert.NoError(t, err)

	// 验证大小已更新
	s, ok := sm.GetSession(session.ID)
	assert.True(t, ok)
	assert.Equal(t, uint16(120), s.Cols)
	assert.Equal(t, uint16(40), s.Rows)

	// 清理
	sm.CloseSession(session.ID)
}

func TestCloseSession(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	sm := NewSessionManager(logger)
	defer sm.Stop()

	req := &CreateSessionRequest{Shell: "/bin/sh"}
	session, err := sm.CreateSession(1, "testuser", req)
	require.NoError(t, err)

	sessionID := session.ID

	// 关闭会话
	err = sm.CloseSession(sessionID)
	assert.NoError(t, err)

	// 验证会话已移除
	_, ok := sm.GetSession(sessionID)
	assert.False(t, ok)

	// 再次关闭应该返回错误
	err = sm.CloseSession(sessionID)
	assert.Error(t, err)
}

func TestSessionToInfo(t *testing.T) {
	session := &Session{
		ID:           "term_test123",
		UserID:       1,
		Username:     "testuser",
		Name:         "Terminal 1",
		Cols:         80,
		Rows:         24,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	info := session.ToInfo()
	assert.Equal(t, session.ID, info.ID)
	assert.Equal(t, session.Name, info.Name)
	assert.Equal(t, session.Cols, info.Cols)
	assert.Equal(t, session.Rows, info.Rows)
	assert.Equal(t, session.CreatedAt, info.CreatedAt)
	assert.Equal(t, session.LastActivity, info.LastActivity)
}

func TestService(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	service := NewService(logger)
	defer service.Stop()

	assert.NotNil(t, service)
	assert.NotNil(t, service.sessionManager)
}
