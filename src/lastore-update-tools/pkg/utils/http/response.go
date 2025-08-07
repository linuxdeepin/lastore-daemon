package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/log"
	"github.com/linuxdeepin/lastore-daemon/src/lastore-update-tools/pkg/utils/ecode"
)

// Response response
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	UUID string      `json:"uuid"` // uuid
	Time string      `json:"time"` // timestamp
	Data interface{} `json:"data"`
}

// OK OK返回正确, data参数可选, body是否带有data字段由data参数决定
func OK(g *gin.Context, data interface{}) {
	respBody := Response{
		Code: ecode.OK,
		Msg:  "",
		Data: data,
	}

	g.JSON(http.StatusOK, respBody)
}

// Fail 返回错误
func Fail(g *gin.Context, errCode int, data interface{}) {
	respBody := Response{
		Code: errCode,
		Msg:  ecode.GetMsg(errCode),
		Data: data,
	}

	g.JSON(http.StatusInternalServerError, respBody)
}

// Option Option
type Option struct {
	Code *int
	Msg  *string
	UUID *string
	Time *string
	Data interface{}
}

// WithSuccess With success option
func WithSuccess() *Option {
	val := 0
	return &Option{Code: &val}
}

// WithCode With code option
func WithCode(code int) *Option {
	str := ecode.GetMsg(code)
	return &Option{Code: &code, Msg: &str}
}

// WithErr With code option
func WithErr(err error) *Option {
	usoCode, ok := err.(*ecode.UosCode)
	if ok {
		return &Option{Code: &usoCode.Code, Msg: &usoCode.ErrorMsg}
	}
	if err != nil {
		return WithCode(ecode.SystemError)
	}
	return WithCode(ecode.OK)
}

// WithFailed With failed option
func WithFailed() *Option {
	return WithCode(1)
}

// WithMsg With msg option
func WithMsg(msg string) *Option {
	return &Option{Msg: &msg}
}

// WithData With data option
func WithData(data interface{}) *Option {
	return &Option{
		Data: data,
	}
}

// JSON JSON Function
func JSON(c *gin.Context, options ...*Option) {
	respBody := Response{
		Code: 0,
		Msg:  "",
		Data: nil,
	}
	for _, option := range options {
		if option.Code != nil && respBody.Code == 0 {
			respBody.Code = *option.Code
		}
		if option.Msg != nil {
			respBody.Msg = *option.Msg
		}
		if option.UUID == nil {
			respBody.UUID = uuid.New().String()
		} else {
			respBody.UUID = *option.UUID
		}
		if option.Time == nil {
			respBody.Time = time.Now().Format(time.RFC3339)
		} else if *option.Time != "" {
			respBody.Time = *option.Time
		}
		if option.Data != nil {
			respBody.Data = option.Data
		}
	}
	log.Debugln("Response", respBody)
	c.JSON(http.StatusOK, respBody)
}
