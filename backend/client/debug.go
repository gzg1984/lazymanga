package client

import (
	"fmt"
)

// CallUpdateEmptyID 调用本地 /debug/updateemptyid 接口
func CallUpdateEmptyID() error {
	resp, err := DoRequest("POST", "/debug/updateemptyid", nil)
	if err != nil {
		return fmt.Errorf("请求失败: %v, 响应: %s", err, string(resp))
	}
	fmt.Printf("/debug/updateemptyid 响应: %s\n", string(resp))
	return nil
}
