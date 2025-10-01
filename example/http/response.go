package main

import (
	"github.com/gin-gonic/gin"
	"github.com/source-build/go-fit"
	"github.com/source-build/go-fit/fres"
)

type PageRequest struct {
	Page        int  `form:"page" json:"page" validate:"required,min=1"`
	PageSize    int  `form:"page_size" json:"page_size" validate:"required,min=5,max=50"`
	ReturnTotal bool `form:"return_total" json:"return_total"`
}

func main() {
	// 注册全局状态码
	fres.RegisterStatusCode(map[interface{}]string{
		10023: "找不到用户信息",
		10024: "身份验证失败",
		10025: "用户信息过期",
		10026: "服务异常",
	})

	// 根据状态码获取描述信息
	fres.StatusCodeDesc(10023)

	// ====================================

	// 快速返回响应信息
	resp, err := internalLogic()
	if err != nil {
		// 对应http状态码 >200
		fres.ErrJson(&gin.Context{}, resp)
	} else {
		// 对应http状态码 == 200
		fres.OkJson(&gin.Context{}, resp)
	}

	// 或

	// 如果 err = nil，那么响应成功(200)，否则返回 400 或 500。
	fres.Response(&gin.Context{}, resp, err)
}

// 服务端错误，对应http状态码 500
func internalLogic() (any, error) {
	// 如果不传第三个参数(err),默认返回包含‘internal server error’的错误信息
	return fres.InternalErrResp(10026, fres.StatusCodeDesc(10026))
	// 返回结果参数
	// fres.InternalErrRespResult(10026, fres.StatusCodeDesc(10026), fit.H{})
	// 快捷返回结果
	// fres.InternalErrRespStatusCode(10026)
	// 快捷返回结果,包含result字段
	// fres.InternalErrRespStatusCode(10026,fit.H{})
}

// 客户端错误，对应http状态码 400
func clientLogic() (any, error) {
	// 如果不传第三个参数(err),默认返回包含‘client error’的错误信息
	return fres.ClientErrResp(fres.StatusClientErr, "用户密码错误")
	// 返回结果参数
	//return fres.ClientErrRespResult(fres.StatusClientErr, "用户密码错误", fit.H{})
}

// 响应成功，对应http状态码 200
func successLogic() (any, error) {
	return fres.OkResp(fres.StatusOK, "用户密码错误", fit.H{"id": 100})
}
