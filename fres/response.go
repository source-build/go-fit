package fres

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

type ResponseOK struct {
	Code   int    `json:"code"`
	Msg    string `json:"msg"`
	Result any    `json:"result"`
}

type ResponseErr struct {
	// 0:client err 1:internal err
	Type   int    `json:"-,omitempty"`
	Code   int    `json:"code"`
	ErrMsg string `json:"err_msg"`
	Result any    `json:"result"`
}

func OkResp(code int, msg string, Result ...interface{}) (ResponseOK, error) {
	r := ResponseOK{Code: code, Msg: msg}

	if len(Result) > 0 {
		r.Result = Result[0]
	}

	return r, nil
}

// InternalErrResp Used to indicate internal server errors, corresponding to an HTTP status code of 500.
// If the 'err' parameter is not displayed, an error message containing 'server error' will be returned by default.
func InternalErrResp(code int, msg string, err ...error) (ResponseErr, error) {
	e := errors.New("internal server error")
	if len(err) > 0 {
		e = err[0]
	}

	return ResponseErr{Code: code, ErrMsg: msg, Type: 1}, e
}

func InternalErrRespStatusCode(code int, err ...error) (ResponseErr, error) {
	r := ResponseErr{Code: code, ErrMsg: StatusCodeDesc(code), Type: 1}

	if len(err) > 0 {
		return r, err[0]
	}

	return r, nil
}

// InternalErrRespResult Similar to InternalErrResp, the difference is the addition of a result field to return data other than descriptive information
func InternalErrRespResult(code int, msg string, result any, err ...error) (ResponseErr, error) {
	e := errors.New("internal server error")
	if len(err) > 0 {
		e = err[0]
	}

	return ResponseErr{Code: code, ErrMsg: msg, Result: result, Type: 1}, e
}

// ClientErrResp Used to indicate a client error with an HTTP status code of 400.
// If the 'err' parameter is not displayed, an error message containing 'server error' will be returned by default.
func ClientErrResp(code int, msg string, err ...error) (ResponseErr, error) {
	e := errors.New("client error")
	if len(err) > 0 {
		e = err[0]
	}

	return ResponseErr{Code: code, ErrMsg: msg, Type: 0}, e
}

// ClientErrRespResult Similar to ClientErrResp, the difference is the addition of a result field to return data other than descriptive information
func ClientErrRespResult(code int, msg string, result any, err ...error) (ResponseErr, error) {
	e := errors.New("client error")
	if len(err) > 0 {
		e = err[0]
	}

	return ResponseErr{Code: code, ErrMsg: msg, Result: result, Type: 0}, e
}

func Response(c *gin.Context, response interface{}, err error) {
	if err == nil {
		OkJson(c, response)
		return
	}

	ErrJson(c, response)
}

// OkJson response success
func OkJson(c *gin.Context, response interface{}) {
	c.JSON(http.StatusOK, response)
}

func ErrJson(c *gin.Context, res interface{}) {
	code := http.StatusInternalServerError
	if v, ok := res.(ResponseErr); v.Type == 0 && ok {
		code = http.StatusBadRequest
	}

	c.JSON(code, res)
}
