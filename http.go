package fit

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
	"unsafe"
)

type HttpUtil struct {
}

func (r HttpUtil) Get(path string, v H) (body []byte, err error) {
	if len(v) > 0 {
		params := url.Values{}
		Url, err := url.Parse(path)
		if err != nil {
			return nil, err
		}
		for k, val := range v {
			params.Set(k, val.(string))
		}
		Url.RawQuery = params.Encode()
		path = Url.String()
	}

	var client http.Client
	var resp *http.Response

	client = http.Client{Timeout: 10 * time.Second}

	resp, err = client.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()
	return body, nil
}

func (r HttpUtil) Post(url string, v H) (string, error) {
	res, err := http.Post(url, "application/json;charset=utf-8", bytes.NewBuffer([]byte(v.ToString())))
	if err != nil {
		return "nil", err
	}
	defer res.Body.Close()

	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	str := (*string)(unsafe.Pointer(&content))
	return *str, nil
}
