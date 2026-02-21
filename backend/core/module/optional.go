package module

// BaseOptionalModule 提供可选模块的默认实现
// 可选模块可以嵌入此结构体来获得默认行为
type BaseOptionalModule struct {
	optional    bool
	description string
	config      []ConfigField
}

// NewBaseOptionalModule 创建基础可选模块
func NewBaseOptionalModule(description string, config []ConfigField) *BaseOptionalModule {
	return &BaseOptionalModule{
		optional:    true,
		description: description,
		config:      config,
	}
}

// IsOptional 返回模块是否可被用户禁用
func (b *BaseOptionalModule) IsOptional() bool {
	return b.optional
}

// Description 返回模块描述
func (b *BaseOptionalModule) Description() string {
	return b.description
}

// DefaultConfig 返回模块的默认配置定义
func (b *BaseOptionalModule) DefaultConfig() []ConfigField {
	return b.config
}

// SetOptional 设置是否可选
func (b *BaseOptionalModule) SetOptional(optional bool) {
	b.optional = optional
}

// SetDescription 设置描述
func (b *BaseOptionalModule) SetDescription(desc string) {
	b.description = desc
}

// SetDefaultConfig 设置默认配置
func (b *BaseOptionalModule) SetDefaultConfig(config []ConfigField) {
	b.config = config
}
