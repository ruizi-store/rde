package bot

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMemory(t *testing.T) *MemoryService {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&BotMemory{}))
	return NewMemoryService(db)
}

func TestMemoryService_SaveAndSearch(t *testing.T) {
	m := setupMemory(t)

	require.NoError(t, m.SaveMemory("b1", "日报格式", "Markdown，含完成/待办/风险三节"))
	require.NoError(t, m.SaveMemory("b1", "负责人", "张三负责前端，李四负责后端"))

	// 同 key 覆盖
	require.NoError(t, m.SaveMemory("b1", "负责人", "张三负责前端，王五负责后端"))
	items, err := m.SearchMemory("b1", "负责人", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "张三负责前端，王五负责后端", items[0].Content)

	// 关键词检索（content 命中）
	items, err = m.SearchMemory("b1", "Markdown", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "日报格式", items[0].Key)

	// 空关键词返回最近条目
	items, err = m.SearchMemory("b1", "", 10)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Bot 隔离
	items, err = m.SearchMemory("b2", "", 10)
	require.NoError(t, err)
	assert.Empty(t, items)

	// 校验
	assert.Error(t, m.SaveMemory("b1", "  ", "x"))
	assert.Error(t, m.SaveMemory("b1", "k", ""))
}

func TestMemoryService_Delete(t *testing.T) {
	m := setupMemory(t)
	require.NoError(t, m.SaveMemory("b1", "k1", "v1"))
	items, err := m.SearchMemory("b1", "", 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// 其他 Bot 不能删
	assert.NoError(t, m.DeleteMemory("b2", items[0].ID))
	items, _ = m.SearchMemory("b1", "", 10)
	assert.Len(t, items, 1)

	assert.NoError(t, m.DeleteMemory("b1", items[0].ID))
	items, _ = m.SearchMemory("b1", "", 10)
	assert.Empty(t, items)
}

func TestBotMemoryProvider_Adapter(t *testing.T) {
	m := setupMemory(t)
	p := &botMemoryProvider{svc: m}

	require.NoError(t, p.SaveMemory("b1", "偏好", "简洁回复"))
	items, err := p.SearchMemory("b1", "偏好", 5)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "偏好", items[0].Key)
	assert.Equal(t, "简洁回复", items[0].Content)
	assert.NotEmpty(t, items[0].UpdatedAt)
}
