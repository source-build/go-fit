package fit

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HttpUtil struct {
	response *http.Response
	Err      error
}

func (r *HttpUtil) GET(path string, v H) *HttpUtil {
	if len(v) > 0 {
		params := url.Values{}
		Url, err := url.Parse(path)
		if err != nil {
			r.Err = err
			return r
		}
		for k, val := range v {
			vl, ok := val.(string)
			if ok {
				params.Set(k, vl)
			}
		}
		Url.RawQuery = params.Encode()
		path = Url.String()
	}

	c := http.Client{Timeout: 10 * time.Second}
	response, err := c.Get(path)
	defer c.CloseIdleConnections()
	if err != nil {
		r.Err = err
		return r
	}

	r.response = response

	return r
}

func (r *HttpUtil) POST(url string, v H, contentType ...string) *HttpUtil {
	ct := "application/json;charset=utf-8"
	if len(contentType) > 0 {
		ct = contentType[0]
	}

	response, err := http.Post(url, ct, bytes.NewBuffer([]byte(v.ToString())))
	if err != nil {
		r.Err = err
		return r
	}

	r.response = response
	return r
}

func (r *HttpUtil) PUT(url string, v H, contentType ...string) *HttpUtil {
	ct := "application/json;charset=utf-8"
	if len(contentType) > 0 {
		ct = contentType[0]
	}

	request, err := http.NewRequest("PUT", url, bytes.NewBuffer([]byte(v.ToString())))
	if err != nil {
		r.Err = err
		return r
	}

	request.Header.Add("Content-Type", ct)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		r.Err = err
		return r
	}
	r.response = response
	return r
}

func (r *HttpUtil) DELETE(url string, v H, contentType ...string) *HttpUtil {
	ct := "application/json;charset=utf-8"
	if len(contentType) > 0 {
		ct = contentType[0]
	}
	request, err := http.NewRequest("DELETE", url, bytes.NewBuffer([]byte(v.ToString())))
	if err != nil {
		r.Err = err
		return r
	}
	request.Header.Add("Content-Type", ct)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		r.Err = err
		return r
	}
	r.response = response
	return r
}

func (r *HttpUtil) PATCH(url string, v H, contentType ...string) *HttpUtil {
	ct := "application/json;charset=utf-8"
	if len(contentType) > 0 {
		ct = contentType[0]
	}
	request, err := http.NewRequest("PATCH", url, bytes.NewBuffer([]byte(v.ToString())))
	if err != nil {
		r.Err = err
		return r
	}
	request.Header.Add("Content-Type", ct)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		r.Err = err
		return r
	}
	r.response = response
	return r
}

func (r *HttpUtil) NewRequest(method string, url string, body string, header ...H) *HttpUtil {
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if len(header) > 0 {
		for k, v := range header[0] {
			request.Header.Add(k, v.(string))
		}
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		r.Err = err
		return r
	}

	r.response = response

	return r
}

func (r *HttpUtil) Body() ([]byte, error) {
	if r.response == nil {
		if r.Err != nil {
			return nil, r.Err
		}
		return nil, errors.New("the response does not exist")
	}

	defer r.response.Body.Close()

	body, err := io.ReadAll(r.response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (r *HttpUtil) Response() (*http.Response, error) {
	if r.response == nil {
		if r.Err != nil {
			return nil, r.Err
		}
		return nil, errors.New("the response does not exist")
	}

	return r.response, nil
}
