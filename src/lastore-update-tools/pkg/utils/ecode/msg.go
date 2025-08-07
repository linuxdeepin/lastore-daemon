package ecode

// MsgFlags msg flags
var MsgFlags = map[int]string{
	OK:                  "请求成功",
	Created:             "成功请求并创建了新的资源",
	NoContent:           "无内容，服务器成功处理，但未返回内容",
	BadRequest:          "请求参数错误",
	Unauthorized:        "请求要求用户的身份认证",
	NotFound:            "未找到资源",
	InternalServerError: "服务器内部错误",

	GetInfoFail:         "用户不存在",
	CreateInfoFail:      "创建用户失败",
	ErrorCheckAdminFail: "用户名或密码错误",

	DatabaseOperationError: "数据库访问错误",

	NotImplemented: "尚未实现",
	SystemError:    "系统错误",
}

// GetMsg 根据代码获取错误信息
func GetMsg(code int) string {
	msg, ok := MsgFlags[code]
	if ok {
		return msg
	}
	return MsgFlags[InternalServerError]
}
