// Package config 中的 file.go 负责从 YAML 配置文件加载受支持的字段。
//
// 刻意只实现扁平 key: value 的最小解析（支持单/双引号、行末 `#` 注释），
// 避免引入第三方 YAML 依赖，保持 tinyCode "1500 行"的极简哲学。
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FileConfig 对应 config.yaml 中受支持的字段。
//
// 字段设计保持最小：基础三项（api_key / base_url / model）
// 与观测开关（trace / trace_dir）。其他运行时参数走 CLI flag / env，避免文件臃肿。
type FileConfig struct {
	APIKey   string
	BaseURL  string
	Model    string
	Trace    bool   // trace: true / 1 / on / yes 视为开启
	TraceDir string // trace_dir: 相对/绝对路径
}

// LoadFile 读取并解析 YAML 配置文件。
//
// 行为：
//   - path 为空 → 返回空 FileConfig，无错误；
//   - 文件不存在 → 返回空 FileConfig，无错误（允许"没有文件"的常见场景）；
//   - 文件格式非法 → 返回错误，便于用户快速定位。
func LoadFile(path string) (FileConfig, error) {
	var empty FileConfig
	if strings.TrimSpace(path) == "" {
		return empty, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return empty, nil
		}
		return empty, err
	}
	defer f.Close()

	cfg := FileConfig{}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := stripComment(scanner.Text())
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		key, val, ok := splitKV(line)
		if !ok {
			return empty, fmt.Errorf("解析 %s:%d 失败：非法行 %q", path, lineNo, line)
		}
		val = unquote(val)
		switch strings.ToLower(key) {
		case "api_key", "apikey":
			cfg.APIKey = val
		case "base_url", "baseurl":
			cfg.BaseURL = val
		case "model":
			cfg.Model = val
		case "trace":
			cfg.Trace = parseYAMLBool(val)
		case "trace_dir", "tracedir":
			cfg.TraceDir = val
		default:
			// 未知字段保持兼容性：忽略但不报错，避免阻塞升级。
		}
	}
	if err := scanner.Err(); err != nil {
		return empty, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	return cfg, nil
}

// stripComment 去掉行末 `#` 注释。
// 只有当 `#` 位于行首或前一字符是空白时才视为注释，兼容形如 "sk-abc#123" 的值。
func stripComment(line string) string {
	inSingle, inDouble := false, false
	var prev rune
	runes := []rune(line)
	for i, r := range runes {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || prev == ' ' || prev == '\t') {
				return string(runes[:i])
			}
		}
		prev = r
	}
	return line
}

// splitKV 以首个冒号拆分 "key: value"。
func splitKV(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// unquote 去除两端成对的单/双引号。
func unquote(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseYAMLBool 解析 YAML 中的布尔表示法。
// 视为 true：true / yes / on / 1（大小写不敏感），其余一律为 false。
func parseYAMLBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}
