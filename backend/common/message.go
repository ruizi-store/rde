package common

// EventType 事件类型定义
type EventType struct {
	Name             string   `json:"name"`
	SourceID         string   `json:"source_id"`
	PropertyTypeList []string `json:"property_type_list"`
}

// 事件类型列表
var EventTypes = []EventType{
	{Name: "rde:system:utilization", SourceID: SERVICENAME, PropertyTypeList: []string{}},
	{Name: "rde:file:recover", SourceID: SERVICENAME, PropertyTypeList: []string{}},
	{Name: "rde:file:operate", SourceID: SERVICENAME, PropertyTypeList: []string{}},
}
