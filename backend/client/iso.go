package client

import (
	"encoding/json"
	"fmt"
	"lazymanga/models"

	"github.com/hokaccha/go-prettyjson"
)

// CallGetISOs 调用本地 /isos 接口并打印结果
func CallGetISOs() error {
	resp, err := DoRequest("GET", "/isos", nil)
	if err != nil {
		return fmt.Errorf("请求/isos失败: %v, 响应: %s", err, string(resp))
	}
	var isos []models.ISOs
	if err := json.Unmarshal(resp, &isos); err != nil {
		return fmt.Errorf("解析/isos响应失败: %v, 原始: %s", err, string(resp))
	}
	pretty, err := prettyjson.Marshal(isos)
	if err != nil {
		return fmt.Errorf("prettyjson 格式化失败: %v", err)
	}
	fmt.Printf("/isos 响应(格式化):\n%s\n", string(pretty))
	return nil
}
