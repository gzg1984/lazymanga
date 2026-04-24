package client

import (
	"fmt"
)

// CallGetUserInfo 调用本地 /userinfo 接口并打印结果
func CallGetUserInfo() error {
	resp, err := DoRequest("GET", "/userinfo", nil)
	if err != nil {
		return fmt.Errorf("请求/userinfo失败: %v, 响应: %s", err, string(resp))
	}
	fmt.Printf("/userinfo 响应: %s\n", string(resp))
	return nil
}
