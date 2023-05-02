package fit

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RequestMethods string

const (
	GET    RequestMethods = "GET"
	POST   RequestMethods = "POST"
	PUT    RequestMethods = "PUT"
	DELETE RequestMethods = "DELETE"
)

type HttpUtil struct {
	response *http.Response
	Err      error
}

func (r *HttpUtil) Get(path string, v H) *HttpUtil {
	if len(v) > 0 {
		params := url.Values{}
		Url, err := url.Parse(path)
		if err != nil {
			r.Err = err
			return r
		}
		for k, val := range v {
			val, ok := val.(string)
			if ok {
				params.Set(k, val)
			}
		}
		Url.RawQuery = params.Encode()
		path = Url.String()
	}

	client := http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(path)
	defer client.CloseIdleConnections()
	if err != nil {
		r.Err = err
		return r
	}
	r.response = response
	return r
}

func (r *HttpUtil) Post(url string, v H) *HttpUtil {
	response, err := http.Post(url, "application/json;charset=utf-8", bytes.NewBuffer([]byte(v.ToString())))
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
		return nil, NewErr("The response does not exist")
	}

	body, err := ioutil.ReadAll(r.response.Body)
	if err != nil {
		return nil, err
	}
	defer r.response.Body.Close()

	return body, nil
}

func (r *HttpUtil) Response() (*http.Response, error) {
	if r.response == nil {
		if r.Err != nil {
			return nil, r.Err
		}
		return nil, NewErr("The response does not exist")
	}

	return r.response, nil
}
