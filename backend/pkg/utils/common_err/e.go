package common_err

// 通用错误码
const (
	// 成功
	SUCCESS = 200

	// 客户端错误 4xx
	CLIENT_ERROR     = 400
	UNAUTHORIZED     = 401
	FORBIDDEN        = 403
	NOT_FOUND        = 404
	METHOD_NOT_ALLOW = 405
	INVALID_PARAMS   = 422

	// 服务端错误 5xx
	ERROR                = 500
	SERVICE_ERROR        = 500
	SERVICE_UNAVAILABLE  = 503
	INTERNAL_SERVER_ERR  = 500

	// 业务错误码 1xxx
	USER_NOT_EXIST       = 1001
	USER_ALREADY_EXIST   = 1002
	PASSWORD_INVALID     = 1003
	TOKEN_INVALID        = 1004
	TOKEN_EXPIRED        = 1005
	
	// 文件相关 2xxx
	FILE_NOT_EXIST       = 2001
	FILE_ALREADY_EXIST   = 2002
	FILE_UPLOAD_FAIL     = 2003
	FILE_DELETE_FAIL     = 2004
	FILE_READ_FAIL       = 2005
	FILE_WRITE_FAIL      = 2006
	DIR_NOT_EXIST        = 2007
	DIR_ALREADY_EXIST    = 2008
	FILE_OR_DIR_EXISTS   = 2009  // CasaOS 兼容
	DIR_ALREADY_EXISTS   = 2008  // CasaOS 兼容（别名）
	
	// CasaOS 兼容错误码
	FILE_DOES_NOT_EXIST  = 2001  // 别名
	FILE_READ_ERROR      = 2005  // 别名
	SOURCE_DES_SAME      = 2010  // 源和目标相同
	FILE_ALREADY_EXISTS  = 2002  // 别名
	
	// 路径相关
	PATH_NOT_EXIST       = 2011
	PATH_IS_NOT_DIR      = 2012
	PATH_IS_NOT_FILE     = 2013
	INSUFFICIENT_PERMISSIONS = 2014
	MOUNTED_DIRECTIORIES = 2015  // 挂载目录
	FILE_DELETE_ERROR    = 2016  // 删除错误
	DIR_NOT_EXISTS       = 2007  // 别名
	
	// 服务相关
	SERVICE_NOT_RUNNING  = 5001
	SERVICE_START_FAIL   = 5002
	SERVICE_STOP_FAIL    = 5003
	
	// 网络与端口相关
	PORT_IS_OCCUPIED     = 5100
	PORT_NOT_AVAILABLE   = 5101
	
	// 共享相关
	SHARE_ALREADY_EXISTS = 6001
	SHARE_NAME_ALREADY_EXISTS = 6002
	SHARE_NOT_EXISTS     = 6003
	Record_ALREADY_EXIST = 6004
	Record_NOT_EXIST     = 6005
	SHARE_PASSWORD_INVALID = 6006
	SHARE_EXPIRED        = 6007
	SHARE_DOWNLOAD_LIMIT = 6008
	
	// 其他
	CHARACTER_LIMIT      = 7001
	NAME_INVALID         = 7002
	
	// Docker 相关 3xxx
	DOCKER_NOT_RUNNING   = 3001
	DOCKER_IMAGE_PULL_FAIL = 3002
	DOCKER_CONTAINER_CREATE_FAIL = 3003
	DOCKER_CONTAINER_START_FAIL  = 3004
	DOCKER_CONTAINER_STOP_FAIL   = 3005
	DOCKER_NETWORK_FAIL  = 3006
	
	// 应用商店 4xxx
	APP_NOT_FOUND        = 4001
	APP_ALREADY_INSTALLED = 4002
	APP_INSTALL_FAIL     = 4003
	APP_UNINSTALL_FAIL   = 4004
	APP_START_FAIL       = 4005
	APP_STOP_FAIL        = 4006
)

// 错误信息映射
var msgMap = map[int]string{
	SUCCESS:              "success",
	CLIENT_ERROR:         "客户端请求错误",
	UNAUTHORIZED:         "未授权，请先登录",
	FORBIDDEN:            "禁止访问",
	NOT_FOUND:            "资源不存在",
	METHOD_NOT_ALLOW:     "方法不允许",
	INVALID_PARAMS:       "参数无效",
	ERROR:                "服务器内部错误",
	SERVICE_UNAVAILABLE:  "服务不可用",

	USER_NOT_EXIST:       "用户不存在",
	USER_ALREADY_EXIST:   "用户已存在",
	PASSWORD_INVALID:     "密码错误",
	TOKEN_INVALID:        "Token 无效",
	TOKEN_EXPIRED:        "Token 已过期",

	FILE_NOT_EXIST:       "文件不存在",
	FILE_ALREADY_EXIST:   "文件已存在",
	FILE_UPLOAD_FAIL:     "文件上传失败",
	FILE_DELETE_FAIL:     "文件删除失败",
	FILE_READ_FAIL:       "文件读取失败",
	FILE_WRITE_FAIL:      "文件写入失败",
	DIR_NOT_EXIST:        "目录不存在",
	DIR_ALREADY_EXIST:    "目录已存在",

	DOCKER_NOT_RUNNING:   "Docker 服务未运行",
	DOCKER_IMAGE_PULL_FAIL: "Docker 镜像拉取失败",
	DOCKER_CONTAINER_CREATE_FAIL: "Docker 容器创建失败",
	DOCKER_CONTAINER_START_FAIL:  "Docker 容器启动失败",
	DOCKER_CONTAINER_STOP_FAIL:   "Docker 容器停止失败",
	DOCKER_NETWORK_FAIL:  "Docker 网络操作失败",

	APP_NOT_FOUND:        "应用不存在",
	APP_ALREADY_INSTALLED: "应用已安装",
	APP_INSTALL_FAIL:     "应用安装失败",
	APP_UNINSTALL_FAIL:   "应用卸载失败",
	APP_START_FAIL:       "应用启动失败",
	APP_STOP_FAIL:        "应用停止失败",
	
	PORT_IS_OCCUPIED:     "端口已被占用",
	PORT_NOT_AVAILABLE:   "端口不可用",
	SERVICE_NOT_RUNNING:  "服务未运行",
	SERVICE_START_FAIL:   "服务启动失败",
	SERVICE_STOP_FAIL:    "服务停止失败",
	
	SHARE_ALREADY_EXISTS: "共享已存在",
	SHARE_NAME_ALREADY_EXISTS: "共享名称已存在",
	SHARE_NOT_EXISTS:     "共享不存在",
	Record_ALREADY_EXIST: "记录已存在",
	Record_NOT_EXIST:     "记录不存在",
	CHARACTER_LIMIT:      "字符超出限制",
	NAME_INVALID:         "名称无效",
}

// GetMsg 根据错误码获取错误信息
func GetMsg(code int) string {
	if msg, ok := msgMap[code]; ok {
		return msg
	}
	return "未知错误"
}

// CodeError 带错误码的错误类型
type CodeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error 实现 error 接口
func (e *CodeError) Error() string {
	return e.Message
}

// NewError 创建错误
func NewError(code int) *CodeError {
	return &CodeError{
		Code:    code,
		Message: GetMsg(code),
	}
}

// NewErrorWithMsg 创建带自定义消息的错误
func NewErrorWithMsg(code int, msg string) *CodeError {
	return &CodeError{
		Code:    code,
		Message: msg,
	}
}

// IsSuccess 检查是否成功
func IsSuccess(code int) bool {
	return code == SUCCESS
}
