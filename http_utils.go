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
)

/**
 * response body status message
 */

const (
	SBusy     = "系统繁忙"
	HandleErr = "操作失败"
	HandleOk  = "操作成功"
)

var HandlerFailErr = errors.New(HandleErr)

const (
	TypeClientErr = iota
	TypeServerErr
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
	//ErrType Mainly divided into client errors and server errors
	ErrType int    `json:"-"`
	Code    int    `json:"code"`
	ErrMsg  string `json:"err_msg"`
	Result  any    `json:"result"`
}

/**
* http response handler
 */

// SvInternalErr http response handler for service internal error,
// corresponding http status code is 500
// for example,db query failed
func SvInternalErr(code int, msg string, err error) (ResponseErr, error) {
	return ResponseErr{Code: code, ErrMsg: msg, ErrType: TypeServerErr}, err
}

func SvInternalErrResult(code int, msg string, result any, err error) (ResponseErr, error) {
	return ResponseErr{Code: code, ErrMsg: msg, Result: result, ErrType: TypeServerErr}, err
}

// ClientErr ClientLogicErr http response handler for client logic error,
// corresponding http status code is 400
func ClientErr(code int, msg string, err error) (ResponseErr, error) {
	return ResponseErr{Code: code, ErrMsg: msg, ErrType: TypeClientErr}, err
}

func ClientErrResult(code int, msg string, result any, err error) (ResponseErr, error) {
	return ResponseErr{Code: code, ErrMsg: msg, Result: result, ErrType: TypeClientErr}, err
}

func JSON(c *gin.Context, responseBody any, err error) {
	if err != nil {
		ErrJson(c, responseBody)
	} else {
		OkJson(c, responseBody)
	}
}

// OkJson response success
func OkJson(c *gin.Context, response interface{}) {
	c.JSON(http.StatusOK, response)
}

// ErrJson client error
func ErrJson(c *gin.Context, response interface{}) {
	var code int
	switch response.(ResponseErr).ErrType {
	case TypeClientErr:
		code = http.StatusBadRequest
	case TypeServerErr:
		code = http.StatusInternalServerError
	}
	c.JSON(code, response)
}

func OkString(c *gin.Context, format string, response interface{}) {
	c.String(http.StatusOK, format, response)
}

func ErrString(c *gin.Context, format string, response interface{}) {
	var code int
	switch response.(ResponseErr).Code {
	case TypeClientErr:
		code = http.StatusBadRequest
	case TypeServerErr:
		code = http.StatusInternalServerError
	}
	c.String(code, format, response)
}
