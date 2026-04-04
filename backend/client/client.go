package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DoRequest 发送本地http请求，支持GET、POST、DELETE等
// method: "GET"/"POST"/"DELETE"，path: "/mount"等，body: 可为nil或map结构体
func DoRequest(method, path string, body interface{}) ([]byte, error) {
	url := "http://127.0.0.1:3000" + path
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}
