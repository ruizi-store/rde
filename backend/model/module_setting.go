package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// ModuleSetting 模块设置
type ModuleSetting struct {
	ModuleID  string    `json:"module_id" gorm:"primaryKey;size:64"`
	Enabled   bool      `json:"enabled" gorm:"default:false"`
	Config    JSONMap   `json:"config" gorm:"type:text"` // 模块配置 JSON
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 返回表名
func (ModuleSetting) TableName() string {
	return "module_settings"
}

// JSONMap 用于存储 JSON 配置
type JSONMap map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("invalid type for JSONMap")
	}

	if len(bytes) == 0 {
		*j = make(JSONMap)
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// ModuleSettingWithMeta 带元信息的模块设置（用于 API 响应）
type ModuleSettingWithMeta struct {
	ModuleID      string        `json:"module_id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Category      string        `json:"category"` // "core" 或 "optional"
	Enabled       bool          `json:"enabled"`
	Dependencies  []string      `json:"dependencies"`
	DepsSatisfied bool          `json:"deps_satisfied"` // 依赖是否满足
	Config        JSONMap       `json:"config"`
	ConfigSchema  []ConfigField `json:"config_schema"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// ConfigField 配置字段定义（用于前端渲染表单）
type ConfigField struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type"` // string/number/bool/select
	Default     interface{} `json:"default"`
	Options     []string    `json:"options,omitempty"` // select 选项
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
}
