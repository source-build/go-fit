package fit

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

/**
 * response body status code
 */

const (
	// StatusSInternalErr service internal error, corresponding http status code is 500
	StatusSInternalErr = 10500
	// StatusCErr client error, corresponding http status code is 400
	StatusCErr = 10400
	// StatusOK success, corresponding http status code is 200
	StatusOK = 0
	// StatusNotJwt jwt not found, corresponding http status code is 400
	StatusNotJwt = 10501
	// AuthFailed authentication failed, corresponding http status code is 400
	AuthFailed = 10504
	// ParamsTypeErr parameter type error, corresponding http status code is 400
	ParamsTypeErr = 10405
)

/**
 * response body status message
 */

const (
	SBusy     = "系统繁忙"
	HandleErr = "操作失败"
	HandleOk  = "操作成功"
)

func NewErr(err string) error {
	return errors.New(err)
}

/**
 * response types
 */

type ResponseOK struct {
	Code   int         `json:"code"`
	Msg    string      `json:"msg"`
	Result interface{} `json:"result"`
}

type ResponseErr struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

/**
* http response handler
 */

// SvInternalErr http response handler for service internal error,
// corresponding http status code is 500
// for example,db query failed
func SvInternalErr(msg string, err error) (ResponseErr, error) {
	return ResponseErr{Code: StatusSInternalErr, Msg: msg}, err
}

// ClientLogicErr http response handler for client logic error,
// corresponding http status code is 400
func ClientLogicErr(msg string, err error) (ResponseErr, error) {
	return ResponseErr{Code: StatusCErr, Msg: msg}, err
}

// OkJson response success
func OkJson(c *gin.Context, response interface{}) {
	c.JSON(http.StatusOK, response)
}

// ErrJson client error
func ErrJson(c *gin.Context, response interface{}) {
	var code int
	switch response.(ResponseErr).Code {
	case StatusCErr:
		code = http.StatusBadRequest
	case StatusSInternalErr:
		code = http.StatusInternalServerError
	}
	c.JSON(code, response)
}

func OKString(c *gin.Context, format string, response interface{}) {
	c.String(http.StatusOK, format, response)
}

func ErrString(c *gin.Context, format string, response interface{}) {
	var code int
	switch response.(ResponseErr).Code {
	case StatusCErr:
		code = http.StatusBadRequest
	case StatusSInternalErr:
		code = http.StatusInternalServerError
	}
	c.String(code, format, response)
}
