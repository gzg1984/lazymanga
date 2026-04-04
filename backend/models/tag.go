package models

import (
	"regexp"
	"strings"
)

// 分词黑名单
var tagBlacklist = map[string]struct{}{
	"iso": {},
}
var specialTags = map[string]struct{}{
	"centos": {}, // 可扩展
	"ubuntu": {},
}

// ExtractTagsFromFileName 从文件名中提取非纯数字单词，返回逗号分隔的tag字符串，支持tag黑名单和特殊tag合并数字
func ExtractTagsFromFileName(baseName string) string {

	re := regexp.MustCompile(`[A-Za-z0-9_]+`)
	words := re.FindAllString(baseName, -1)
	var tags []string
	for i := 0; i < len(words); i++ {
		w := words[i]
		if regexp.MustCompile(`^\d+$`).MatchString(w) {
			continue
		}
		lw := strings.ToLower(w)
		if _, black := tagBlacklist[lw]; black {
			continue
		}
		tags = append(tags, w)
		// 特殊tag合并后数字
		if _, special := specialTags[lw]; special && i+1 < len(words) {
			next := words[i+1]
			if regexp.MustCompile(`^\d+$`).MatchString(next) {
				tags = append(tags, w+"-"+next)
			}
		}
	}
	return strings.Join(tags, ",")
}
