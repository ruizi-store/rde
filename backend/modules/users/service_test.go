package users

import (
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&User{}, &UserGroup{})
	require.NoError(t, err)

	return db
}

func TestService_CreateUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建用户
	err := svc.CreateUser(&CreateUserRequest{
		Username: "testuser",
		Password: "password123",
		Email:    "test@example.com",
		Nickname: "Test User",
	})
	require.NoError(t, err)

	// 获取用户
	user, err := svc.GetUserByUsername("testuser")
	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.Nickname)
	assert.Equal(t, RoleUser, user.Role)
	assert.Equal(t, StatusActive, user.Status)

	// 重复创建应失败
	err = svc.CreateUser(&CreateUserRequest{
		Username: "testuser",
		Password: "password123",
	})
	assert.Equal(t, ErrUserExists, err)
}

func TestService_ValidatePassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建用户
	err := svc.CreateUser(&CreateUserRequest{
		Username: "testuser",
		Password: "password123",
	})
	require.NoError(t, err)

	// 验证正确密码
	user, err := svc.ValidatePassword("testuser", "password123")
	require.NoError(t, err)
	assert.Equal(t, "testuser", user.Username)
	assert.NotNil(t, user.LastLogin)

	// 验证错误密码
	_, err = svc.ValidatePassword("testuser", "wrongpassword")
	assert.Equal(t, ErrInvalidPassword, err)

	// 验证不存在的用户
	_, err = svc.ValidatePassword("nonexistent", "password123")
	assert.Equal(t, ErrUserNotFound, err)
}

func TestService_ChangePassword(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建用户
	err := svc.CreateUser(&CreateUserRequest{
		Username: "testuser",
		Password: "oldpassword",
	})
	require.NoError(t, err)

	user, _ := svc.GetUserByUsername("testuser")

	// 使用错误的旧密码
	err = svc.ChangePassword(user.ID, "wrongold", "newpassword")
	assert.Equal(t, ErrInvalidPassword, err)

	// 使用正确的旧密码
	err = svc.ChangePassword(user.ID, "oldpassword", "newpassword")
	require.NoError(t, err)

	// 验证新密码
	_, err = svc.ValidatePassword("testuser", "newpassword")
	require.NoError(t, err)
}

func TestService_UpdateUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建用户
	err := svc.CreateUser(&CreateUserRequest{
		Username: "testuser",
		Password: "password123",
	})
	require.NoError(t, err)

	user, _ := svc.GetUserByUsername("testuser")

	// 更新用户
	err = svc.UpdateUser(user.ID, &UpdateUserRequest{
		Email:    "updated@example.com",
		Nickname: "Updated Name",
		Role:     RoleAdmin,
	})
	require.NoError(t, err)

	// 验证更新
	updatedUser, _ := svc.GetUserByID(user.ID)
	assert.Equal(t, "updated@example.com", updatedUser.Email)
	assert.Equal(t, "Updated Name", updatedUser.Nickname)
	assert.Equal(t, RoleAdmin, updatedUser.Role)
}

func TestService_DeleteUser(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建普通用户
	err := svc.CreateUser(&CreateUserRequest{
		Username: "normaluser",
		Password: "password123",
	})
	require.NoError(t, err)

	user, _ := svc.GetUserByUsername("normaluser")

	// 删除普通用户
	err = svc.DeleteUser(user.ID)
	require.NoError(t, err)

	// 确认已删除
	_, err = svc.GetUserByID(user.ID)
	assert.Equal(t, ErrUserNotFound, err)
}

func TestService_CannotDeleteOnlyAdmin(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建管理员
	err := svc.CreateUser(&CreateUserRequest{
		Username: "admin",
		Password: "password123",
		Role:     RoleAdmin,
	})
	require.NoError(t, err)

	admin, _ := svc.GetUserByUsername("admin")

	// 尝试删除唯一的管理员应该失败
	err = svc.DeleteUser(admin.ID)
	assert.Equal(t, ErrCannotDeleteAdmin, err)

	// 创建另一个管理员
	err = svc.CreateUser(&CreateUserRequest{
		Username: "admin2",
		Password: "password123",
		Role:     RoleAdmin,
	})
	require.NoError(t, err)

	// 现在可以删除第一个管理员
	err = svc.DeleteUser(admin.ID)
	require.NoError(t, err)
}

func TestService_ListUsers(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建多个用户
	for i := 0; i < 5; i++ {
		err := svc.CreateUser(&CreateUserRequest{
			Username: "user" + string(rune('a'+i)),
			Password: "password123",
		})
		require.NoError(t, err)
	}

	// 获取全部
	users, total, err := svc.ListUsers(0, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, users, 5)

	// 分页获取
	users, total, err = svc.ListUsers(1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, users, 2)
}

func TestService_UserGroups(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 创建用户组
	group, err := svc.CreateGroup(&CreateGroupRequest{
		Name:        "Developers",
		Description: "Development team",
	})
	require.NoError(t, err)
	assert.Equal(t, "Developers", group.Name)

	// 重复创建应失败
	_, err = svc.CreateGroup(&CreateGroupRequest{
		Name: "Developers",
	})
	assert.Equal(t, ErrGroupExists, err)

	// 更新用户组
	err = svc.UpdateGroup(group.ID, &UpdateGroupRequest{
		Description: "Updated description",
	})
	require.NoError(t, err)

	updatedGroup, _ := svc.GetGroupByID(group.ID)
	assert.Equal(t, "Updated description", updatedGroup.Description)

	// 创建用户并加入用户组
	err = svc.CreateUser(&CreateUserRequest{
		Username: "devuser",
		Password: "password123",
		GroupID:  group.ID,
	})
	require.NoError(t, err)

	// 获取用户组列表
	groups, err := svc.ListGroups()
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Len(t, groups[0].Users, 1)

	// 删除用户组
	err = svc.DeleteGroup(group.ID)
	require.NoError(t, err)

	// 用户的 group_id 应该被清空
	user, _ := svc.GetUserByUsername("devuser")
	assert.Empty(t, user.GroupID)
}

func TestService_EnsureAdminExists(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, zap.NewNop())

	// 没有管理员时调用不应报错（只记录警告）
	err := svc.EnsureAdminExists()
	require.NoError(t, err)

	// 手动创建管理员后再调用也不应报错
	err = svc.CreateUser(&CreateUserRequest{
		Username: "admin",
		Password: "admin123",
		Role:     RoleAdmin,
	})
	require.NoError(t, err)

	err = svc.EnsureAdminExists()
	require.NoError(t, err)

	admin, err := svc.GetUserByUsername("admin")
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, admin.Role)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
