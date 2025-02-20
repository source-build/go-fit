package fres

// Provided a reference business status code, it should be noted that this status code is not the one in HTTP.
const (
	// StatusInternalErr service internal error, corresponding http status code is 500
	StatusInternalErr = 10500

	// StatusClientErr client error, corresponding http status code is 400
	StatusClientErr = 10400

	// StatusOK success, corresponding http status code is 200
	StatusOK = 0
)

type StatusCode struct {
	codes map[interface{}]string
}

var statusCode *StatusCode

func RegisterStatusCode(codes map[interface{}]string) {
	statusCode = &StatusCode{codes: codes}
}

// StatusCodeDesc Obtain descriptive information based on the status code
func StatusCodeDesc(code interface{}) string {
	if statusCode == nil {
		return ""
	}

	return statusCode.codes[code]
}
