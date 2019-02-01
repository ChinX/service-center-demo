package restful

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

func DoRequest(method string, addr string, header http.Header, body interface{}) (*http.Response, error) {
	r, err := toReader(body)
	if err != nil {
		return nil, fmt.Errorf(" request body wrong: %s", err)
	}

	log.Println(addr)
	req, err := http.NewRequest(method, addr, r)
	if err != nil {
		return nil, err
	}

	if header != nil {
		req.Header = header
	}

	client := http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		},
	}
	return client.Do(req)
}

func ParseResponse(resp *http.Response, expectCode int, expectData interface{}) error {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response faild: %s", err)
	}

	if resp.StatusCode == expectCode {
		if expectData != nil {
			log.Println(string(body))
			switch v := expectData.(type) {
			case io.Writer:
				_, err = v.Write(body)
			case *string:
				*v = string(body)
			case []byte:
				expectData = body
			default:
				err = json.Unmarshal(body, expectData)
			}
			if err != nil {
				return fmt.Errorf("parse response body: \"%s\" faild: %s", string(body), err)
			}
		}
		return nil
	}
	return fmt.Errorf("do request failed, response statusCode: %d, body: %s",
		resp.StatusCode, string(body))
}

func toReader(body interface{}) (r io.Reader, err error) {
	if body != nil {
		switch v := body.(type) {
		case io.Reader:
			r = v
		case string:
			r = strings.NewReader(v)
		case []byte:
			r = bytes.NewReader(v)
		default:
			var data []byte
			data, err = json.Marshal(v)
			if err == nil {
				r = bytes.NewReader(data)
			}
			log.Println(string(data))
		}
	}
	return
}
