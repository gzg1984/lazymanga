package client

import (
	"encoding/json"
	"fmt"
)

// CallQueryAllTags 调用 /queryalltags 接口并打印字符串数组结果
func CallQueryAllTags() error {
	resp, err := DoRequest("GET", "/queryalltags", nil)
	if err != nil {
		return fmt.Errorf("请求/queryalltags失败: %v, 响应: %s", err, string(resp))
	}
	var tags []string
	if err := json.Unmarshal(resp, &tags); err != nil {
		return fmt.Errorf("解析/queryalltags响应失败: %v, 原始: %s", err, string(resp))
	}
	fmt.Printf("/queryalltags 响应: %v\n", tags)
	return nil
}
