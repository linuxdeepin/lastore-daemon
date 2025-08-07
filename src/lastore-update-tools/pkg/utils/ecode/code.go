package ecode

// 负数的错误码直接弹框进行展示。如创建用户，用户参数缺失
// 正数错误码前端进行拦截处理，如果前端没有进行拦截那么不做任何提示。如创建用户信息，用户已经存在
// 业务错误码统一长度6位数，前3位是业务模块，后3位具体业务

const (
	// OK 请求成功。一般用于GET与POST请求
	OK = 0
	// Created 已创建。成功请求并创建了新的资源
	Created = -201
	// NoContent 无内容。服务器成功处理，但未返回内容。在未更新网页的情况下，可确保浏览器继续显示当前文档
	NoContent = -204
	// BadRequest 请求参数错误
	BadRequest = -400
	// Unauthorized 请求要求用户的身份认证
	Unauthorized = -401
	// NotFound 服务器无法根据客户端的请求找到资源（网页）。通过此代码，网站设计人员可设置"您所请求的资源无法找到"的个性页面
	NotFound = -404
	// InternalServerError 服务器内部错误，无法完成请求
	InternalServerError = -500
	// DatabaseOperationError 数据库访问错误
	DatabaseOperationError = 100500
	// ErrorCheckAdminFail 登录
	ErrorCheckAdminFail = -200000 // 用户名或密码错误
	// NotImplemented 尚未实现
	NotImplemented = 100444
	// SystemError 系统错误
	SystemError = 100520
)

// 用户业务
const (
	// GetInfoFail 用户不存在
	GetInfoFail = 100000
	// CreateInfoFail 创建用户失败
	CreateInfoFail = -100001
)

var (
	// ErrBadRequest ErrBadRequest
	ErrBadRequest = NewUosCode(BadRequest, GetMsg(BadRequest), "您好，您的输入有误，请重新输入") //errors.New(GetMsg(BadRequest))
	// ErrUnauthorized ErrUnauthorized
	ErrUnauthorized = NewUosCode(Unauthorized, GetMsg(Unauthorized), "您的系统开小差了，请稍后再试") //errors.New(GetMsg(Unauthorized))
	// ErrNotFound ErrRecordNotFound
	ErrNotFound = NewUosCode(NotFound, GetMsg(NotFound), "您的系统开小差了，请稍后再试") //errors.New(GetMsg(NotFound))
	// ErrDatabaseOperationError ErrDatabaseOperationError
	ErrDatabaseOperationError = NewUosCode(DatabaseOperationError, GetMsg(DatabaseOperationError), "您的系统开小差了，请稍后再试") //errors.New(GetMsg(DatabaseOperationError))
	// ErrNotImplemented ErrNotImplemented
	ErrNotImplemented = NewUosCode(NotImplemented, GetMsg(NotImplemented), "您的系统开小差了，请稍后再试") //errors.New(GetMsg(NotImplemented))
	// ErrSystemError ErrSystemError
	ErrSystemError = NewUosCode(SystemError, GetMsg(SystemError), "您的系统开小差了，请稍后再试") //errors.New(GetMsg(SystemError))

	//todo 不建议将code和error 分别在两处用常量或者map 定义，对阅读代码不友好，用一下定义即可
	ErrInnerRedis = NewUosCode(SystemError, "redis出错", "您的系统开小差了，请稍后再试")
)

//实现 impl go error
type UosCode struct {
	Code     int    `json:"code"`
	ExtCode  int64  `json:"extcode" default:0` 
	ErrorMsg string `json:"error_msg"` //TODO 系统内部错误 ，如果上游是系统内部服务，则可以用此提示，如果上游是对用户的则可以使用toast
	ToastMsg string `json:"toast"`     //友好提示
}


//NewUosCode new一个自定义error
func NewUosCode(code int, errorMsg string, toastMsg ...string) *UosCode {
	var tMsg string
	if len(toastMsg) > 0 {
		tMsg = toastMsg[0]
	}
	return &UosCode{
		Code:     code,
		ErrorMsg: errorMsg,
		ToastMsg: tMsg,
		ExtCode:  0,
	}
}

func (u *UosCode) SetExtCode(code int, extcode int64, extmsg string) {
	u.Code = code
	u.ExtCode = extcode
	u.ErrorMsg = GetMsg(code)
	u.ToastMsg = extmsg
}
func (u *UosCode) Error() string {
	return u.ErrorMsg
}

func (u *UosCode) Toast() error {
	u.ErrorMsg = u.ToastMsg
	return u
}
