// Package download aria2 JSON-RPC 客户端
package download

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Aria2Client aria2 JSON-RPC 客户端
type Aria2Client struct {
	url       string
	secret    string
	client    *http.Client
}

// NewAria2Client 创建 aria2 客户端
func NewAria2Client(url, secret string) *Aria2Client {
	return &Aria2Client{
		url:    url,
		secret: secret,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// call 调用 RPC 方法
func (c *Aria2Client) call(method string, params ...interface{}) (interface{}, error) {
	token := "token:" + c.secret
	allParams := []interface{}{token}
	allParams = append(allParams, params...)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      uuid.New().String(),
		"method":  method,
		"params":  allParams,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Post(c.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Aria2Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Error != nil {
		return nil, fmt.Errorf("aria2 error %d: %s", result.Error.Code, result.Error.Message)
	}

	return result.Result, nil
}

// AddURI 添加 URI 下载
func (c *Aria2Client) AddURI(uris []string, options map[string]interface{}) (string, error) {
	if options == nil {
		options = make(map[string]interface{})
	}
	result, err := c.call("aria2.addUri", uris, options)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// AddTorrent 添加种子下载
func (c *Aria2Client) AddTorrent(torrentBase64 string, options map[string]interface{}) (string, error) {
	if options == nil {
		options = make(map[string]interface{})
	}
	result, err := c.call("aria2.addTorrent", torrentBase64, []interface{}{}, options)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// AddMetalink 添加 Metalink 下载
func (c *Aria2Client) AddMetalink(metalinkBase64 string, options map[string]interface{}) ([]string, error) {
	if options == nil {
		options = make(map[string]interface{})
	}
	result, err := c.call("aria2.addMetalink", metalinkBase64, options)
	if err != nil {
		return nil, err
	}
	gids := make([]string, 0)
	for _, v := range result.([]interface{}) {
		gids = append(gids, v.(string))
	}
	return gids, nil
}

// Pause 暂停任务
func (c *Aria2Client) Pause(gid string) error {
	_, err := c.call("aria2.pause", gid)
	return err
}

// PauseAll 暂停所有任务
func (c *Aria2Client) PauseAll() error {
	_, err := c.call("aria2.pauseAll")
	return err
}

// Unpause 恢复任务
func (c *Aria2Client) Unpause(gid string) error {
	_, err := c.call("aria2.unpause", gid)
	return err
}

// UnpauseAll 恢复所有任务
func (c *Aria2Client) UnpauseAll() error {
	_, err := c.call("aria2.unpauseAll")
	return err
}

// Remove 移除任务
func (c *Aria2Client) Remove(gid string) error {
	_, err := c.call("aria2.remove", gid)
	return err
}

// ForceRemove 强制移除任务
func (c *Aria2Client) ForceRemove(gid string) error {
	_, err := c.call("aria2.forceRemove", gid)
	return err
}

// TellStatus 获取任务状态
func (c *Aria2Client) TellStatus(gid string) (*Aria2TaskStatus, error) {
	result, err := c.call("aria2.tellStatus", gid)
	if err != nil {
		return nil, err
	}
	return c.parseTaskStatus(result)
}

// TellActive 获取活动任务
func (c *Aria2Client) TellActive() ([]*Aria2TaskStatus, error) {
	result, err := c.call("aria2.tellActive")
	if err != nil {
		return nil, err
	}
	return c.parseTaskList(result)
}

// TellWaiting 获取等待任务
func (c *Aria2Client) TellWaiting(offset, num int) ([]*Aria2TaskStatus, error) {
	result, err := c.call("aria2.tellWaiting", offset, num)
	if err != nil {
		return nil, err
	}
	return c.parseTaskList(result)
}

// TellStopped 获取已停止任务
func (c *Aria2Client) TellStopped(offset, num int) ([]*Aria2TaskStatus, error) {
	result, err := c.call("aria2.tellStopped", offset, num)
	if err != nil {
		return nil, err
	}
	return c.parseTaskList(result)
}

// GetGlobalStat 获取全局统计
func (c *Aria2Client) GetGlobalStat() (*Aria2GlobalStat, error) {
	result, err := c.call("aria2.getGlobalStat")
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	var stat Aria2GlobalStat
	if err := json.Unmarshal(data, &stat); err != nil {
		return nil, err
	}
	return &stat, nil
}

// ChangeGlobalOption 修改全局选项
func (c *Aria2Client) ChangeGlobalOption(options map[string]interface{}) error {
	_, err := c.call("aria2.changeGlobalOption", options)
	return err
}

// PurgeDownloadResult 清除下载结果
func (c *Aria2Client) PurgeDownloadResult() error {
	_, err := c.call("aria2.purgeDownloadResult")
	return err
}

// RemoveDownloadResult 移除下载结果
func (c *Aria2Client) RemoveDownloadResult(gid string) error {
	_, err := c.call("aria2.removeDownloadResult", gid)
	return err
}

// Shutdown 关闭 aria2
func (c *Aria2Client) Shutdown() error {
	_, err := c.call("aria2.shutdown")
	return err
}

// GetVersion 获取版本
func (c *Aria2Client) GetVersion() (string, error) {
	result, err := c.call("aria2.getVersion")
	if err != nil {
		return "", err
	}
	m := result.(map[string]interface{})
	if v, ok := m["version"]; ok {
		return v.(string), nil
	}
	return "", nil
}

func (c *Aria2Client) parseTaskStatus(data interface{}) (*Aria2TaskStatus, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var status Aria2TaskStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Aria2Client) parseTaskList(data interface{}) ([]*Aria2TaskStatus, error) {
	list := data.([]interface{})
	tasks := make([]*Aria2TaskStatus, 0, len(list))

	for _, item := range list {
		task, err := c.parseTaskStatus(item)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ParseInt 解析字符串为 int64
func ParseInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// ParseIntToInt 解析字符串为 int
func ParseIntToInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
