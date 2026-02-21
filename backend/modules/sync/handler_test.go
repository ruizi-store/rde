package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ruizi-store/rde/backend/testutil"
)

func setupTestHandler(t *testing.T) (*Handler, *gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	logger := testutil.TestLogger()
	db := testutil.SetupTestDB(t, &SyncFileModel{}, &UploadSessionModel{})
	dataDir := t.TempDir()

	service := NewService(logger, db, dataDir)
	require.NoError(t, service.Init())

	handler := NewHandler(service)
	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, router, service
}

func TestHandler_GetStatus(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))
	data := resp["data"].(map[string]interface{})
	assert.True(t, data["running"].(bool))
}

func TestHandler_ListFiles_Empty(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/files", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))
	assert.Equal(t, float64(0), resp["total"])
	assert.Empty(t, resp["data"])
}

func TestHandler_ListFiles_WithData(t *testing.T) {
	_, router, service := setupTestHandler(t)

	// 插入测试数据
	testFiles := []SyncFileModel{
		{ID: "file-a", Filename: "img1.png", Size: 1000, Status: "completed", CreatedAt: time.Now()},
		{ID: "file-b", Filename: "img2.png", Size: 2000, Status: "completed", CreatedAt: time.Now()},
	}
	for _, f := range testFiles {
		service.db.Create(&f)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/files?limit=10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))
	assert.Equal(t, float64(2), resp["total"])

	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestHandler_GetFile_Found(t *testing.T) {
	_, router, service := setupTestHandler(t)

	// 插入测试数据
	testFile := SyncFileModel{
		ID:       "get-file-test",
		Filename: "photo.jpg",
		Size:     4096,
		MimeType: "image/jpeg",
		Status:   "completed",
	}
	service.db.Create(&testFile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/files/get-file-test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "get-file-test", data["id"])
	assert.Equal(t, "photo.jpg", data["filename"])
}

func TestHandler_GetFile_NotFound(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/files/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.False(t, resp["success"].(bool))
}

func TestHandler_DeleteFile(t *testing.T) {
	_, router, service := setupTestHandler(t)

	// 插入测试数据
	testFile := SyncFileModel{
		ID:       "delete-handler-test",
		Filename: "todelete.txt",
		Size:     100,
		Path:     "/tmp/nonexistent", // 不存在的路径
		Status:   "completed",
	}
	service.db.Create(&testFile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sync/files/delete-handler-test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))

	// 验证已从数据库删除
	var count int64
	service.db.Model(&SyncFileModel{}).Where("id = ?", "delete-handler-test").Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestHandler_DeleteFile_NotFound(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sync/files/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_ListUploads(t *testing.T) {
	_, router, service := setupTestHandler(t)

	// 插入上传会话
	sessions := []UploadSessionModel{
		{ID: "up-1", Filename: "file1.zip", Size: 100000, Offset: 50000, ExpiresAt: time.Now().Add(1 * time.Hour)},
		{ID: "up-2", Filename: "file2.zip", Size: 200000, Offset: 100000, ExpiresAt: time.Now().Add(2 * time.Hour)},
	}
	for _, s := range sessions {
		service.db.Create(&s)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/uploads", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))
	data := resp["data"].([]interface{})
	assert.Len(t, data, 2)
}

func TestHandler_GetUpload_Found(t *testing.T) {
	_, router, service := setupTestHandler(t)

	session := UploadSessionModel{
		ID:        "get-upload-test",
		Filename:  "video.mp4",
		Size:      1000000,
		Offset:    250000,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	service.db.Create(&session)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/uploads/get-upload-test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.True(t, resp["success"].(bool))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "get-upload-test", data["id"])
	assert.Equal(t, 0.25, data["progress"])
}

func TestHandler_GetUpload_NotFound(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync/uploads/non-existent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_TusOptions(t *testing.T) {
	_, router, _ := setupTestHandler(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/sync/upload", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "1.0.0", w.Header().Get("Tus-Resumable"))
	assert.Equal(t, "1.0.0", w.Header().Get("Tus-Version"))
	assert.Contains(t, w.Header().Get("Tus-Extension"), "creation")
}
