package common_err

import (
	"testing"
)

func TestErrorCodes(t *testing.T) {
	// 测试错误码常量是否正确定义
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"SUCCESS", SUCCESS, 200},
		{"CLIENT_ERROR", CLIENT_ERROR, 400},
		{"UNAUTHORIZED", UNAUTHORIZED, 401},
		{"FORBIDDEN", FORBIDDEN, 403},
		{"NOT_FOUND", NOT_FOUND, 404},
		{"ERROR", ERROR, 500},
		{"USER_NOT_EXIST", USER_NOT_EXIST, 1001},
		{"FILE_NOT_EXIST", FILE_NOT_EXIST, 2001},
		{"DOCKER_NOT_RUNNING", DOCKER_NOT_RUNNING, 3001},
		{"APP_NOT_FOUND", APP_NOT_FOUND, 4001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("错误码 %s = %d, 期望 %d", tt.name, tt.code, tt.expected)
			}
		})
	}
}

func TestGetMsg(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{SUCCESS, "success"},
		{CLIENT_ERROR, "客户端请求错误"},
		{UNAUTHORIZED, "未授权，请先登录"},
		{NOT_FOUND, "资源不存在"},
		{ERROR, "服务器内部错误"},
		{USER_NOT_EXIST, "用户不存在"},
		{FILE_NOT_EXIST, "文件不存在"},
		{DOCKER_NOT_RUNNING, "Docker 服务未运行"},
		{APP_NOT_FOUND, "应用不存在"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			msg := GetMsg(tt.code)
			if msg != tt.expected {
				t.Errorf("GetMsg(%d) = %s, 期望 %s", tt.code, msg, tt.expected)
			}
		})
	}
}

func TestGetMsgUnknown(t *testing.T) {
	// 测试未知错误码
	msg := GetMsg(99999)
	if msg != "未知错误" {
		t.Errorf("未知错误码应返回 '未知错误', 实际返回 %s", msg)
	}
}

func TestCodeError(t *testing.T) {
	// 测试 CodeError 类型
	err := NewError(USER_NOT_EXIST)
	
	if err.Code != USER_NOT_EXIST {
		t.Errorf("错误码应为 %d, 实际为 %d", USER_NOT_EXIST, err.Code)
	}
	
	if err.Error() != "用户不存在" {
		t.Errorf("错误信息应为 '用户不存在', 实际为 %s", err.Error())
	}
}

func TestNewErrorWithMsg(t *testing.T) {
	customMsg := "自定义错误消息"
	err := NewErrorWithMsg(ERROR, customMsg)
	
	if err.Code != ERROR {
		t.Errorf("错误码应为 %d, 实际为 %d", ERROR, err.Code)
	}
	
	if err.Message != customMsg {
		t.Errorf("错误信息应为 '%s', 实际为 %s", customMsg, err.Message)
	}
}

func TestCasaOSCompatibility(t *testing.T) {
	// 测试 CasaOS 兼容别名
	if FILE_DOES_NOT_EXIST != FILE_NOT_EXIST {
		t.Error("FILE_DOES_NOT_EXIST 应该等于 FILE_NOT_EXIST")
	}
	
	if FILE_READ_ERROR != FILE_READ_FAIL {
		t.Error("FILE_READ_ERROR 应该等于 FILE_READ_FAIL")
	}
	
	if DIR_NOT_EXISTS != DIR_NOT_EXIST {
		t.Error("DIR_NOT_EXISTS 应该等于 DIR_NOT_EXIST")
	}
	
	if DIR_ALREADY_EXISTS != DIR_ALREADY_EXIST {
		t.Error("DIR_ALREADY_EXISTS 应该等于 DIR_ALREADY_EXIST")
	}
}
