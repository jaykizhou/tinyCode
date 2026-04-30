// Package openai 的 observer.go 提供一个可插拔的「模型交互观测器」。
//
// 设计动机：
//   - 与大模型交互时最常见的故障（400/500/超时/格式不匹配）都发生在 HTTP 边界，
//     而控制台通常只能看到摘要错误；没有完整 payload / body，排查几乎全靠猜。
//   - 为此在 openai.Client 的请求前、响应后、出错时插入观测钩子，
//     把每次交互原样记录到 JSONL（每行一条 JSON），便于事后用 jq / grep 分析。
//
// 设计要点：
//   - Observer 是一个接口，默认 NopObserver 不做任何事，核心代码零感知。
//   - JSONLFileObserver 追加写文件，单次交互一条记录，不做缓冲以防进程崩溃丢数据。
//   - 自动脱敏 Authorization 头，避免日志泄露 API Key。
//   - 写盘失败降级到 stderr 打印，绝不影响主流程。
//   - 【懒创建】文件在第一次实际写入时才创建，避免启动后无交互留下大量空文件。
package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Observer 观测一次 Chat Completions 调用的三个关键时刻。
//
// 实现方需自处理并发（Client 虽然不在同一次请求内并发调用这三个方法，
// 但多个 goroutine 可能共享同一个 Observer）。
type Observer interface {
	// OnRequest 在 HTTP 请求发出前调用。
	//   - url：完整的 chat/completions URL
	//   - headers：最终的请求头（Authorization 建议在实现内脱敏）
	//   - payload：序列化后的请求体 JSON 字节
	OnRequest(url string, headers http.Header, payload []byte)

	// OnResponse 在成功收到响应（无论状态码）后调用。
	//   - status：HTTP 状态码
	//   - body：响应体字节
	//   - duration：请求耗时
	OnResponse(status int, body []byte, duration time.Duration)

	// OnError 在发送请求或读响应阶段发生传输层错误时调用。
	OnError(err error, duration time.Duration)
}

// NopObserver 是默认的空实现，便于 Client 内部调用时无需做 nil 检查。
type NopObserver struct{}

func (NopObserver) OnRequest(string, http.Header, []byte) {}
func (NopObserver) OnResponse(int, []byte, time.Duration) {}
func (NopObserver) OnError(error, time.Duration)          {}

// 编译期断言。
var _ Observer = NopObserver{}

// ------------------------------ JSONL 文件实现 ------------------------------

// JSONLFileObserver 将每次交互以一行 JSON 的形式追加到文件。
//
// 文件格式：每行一条记录，字段包括 ts / kind / (url|status|error) / ...。
// 使用 JSONL（而非单个大 JSON 对象）是为了支持「边跑边追加」以及
// 事后用 jq 快速切片分析。
//
// 【懒创建】：文件在第一次实际写入时才创建，若整个会话没有任何 API 调用，
// 则不会在磁盘上留下空文件。
type JSONLFileObserver struct {
	mu   sync.Mutex // 保护 f 的串行写入及懒初始化
	f    *os.File
	dir  string // 目标目录，懒创建时使用
	name string // 文件名，懒创建时使用
	path string // 完整路径，首次创建后固定
}

// NewJSONLFileObserver 创建一个文件观测器。
//
// dir 不存在会在首次写入时自动创建（o755）。
// 文件名固定为 openai-YYYYMMDD-HHMMSS.jsonl，保证单次进程一个独立文件，
// 便于对照一次运行排查问题。
//
// 注意：此函数不再立即创建文件，文件将在第一次写入时才真正落盘。
func NewJSONLFileObserver(dir string) (*JSONLFileObserver, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("trace dir 不能为空")
	}
	name := fmt.Sprintf("openai-%s.jsonl", time.Now().Format("20060102-150405"))
	return &JSONLFileObserver{
		dir:  dir,
		name: name,
		path: filepath.Join(dir, name),
	}, nil
}

// Path 返回当前写入的文件绝对路径，便于在 UI 中展示「trace -> xxx.jsonl」。
// 注意：文件可能尚未创建（无交互时），调用方仅用于展示即可。
func (o *JSONLFileObserver) Path() string { return o.path }

// Close 关闭底层文件；Observer 生命周期与 Client 绑定时应在进程退出前调用。
// 若文件从未被创建（无任何写入），Close 是无操作的。
func (o *JSONLFileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.f == nil {
		return nil
	}
	err := o.f.Close()
	o.f = nil
	return err
}

// OnRequest 记录请求。payload 原样写入，不做截断——排查问题时宁可文件大些，也不愿丢字段。
func (o *JSONLFileObserver) OnRequest(url string, headers http.Header, payload []byte) {
	o.write(map[string]any{
		"ts":      nowRFC3339(),
		"kind":    "request",
		"url":     url,
		"headers": redactHeaders(headers),
		"payload": json.RawMessage(ensureJSON(payload)),
	})
}

// OnResponse 记录响应。
// body 若是合法 JSON，作为结构化字段写入（便于 jq 取值）；
// 否则退化为字符串字段。
func (o *JSONLFileObserver) OnResponse(status int, body []byte, duration time.Duration) {
	record := map[string]any{
		"ts":          nowRFC3339(),
		"kind":        "response",
		"status":      status,
		"duration_ms": duration.Milliseconds(),
	}
	if isJSON(body) {
		record["body"] = json.RawMessage(body)
	} else {
		record["body"] = string(body)
	}
	o.write(record)
}

// OnError 记录传输层错误（DNS 解析、连接失败、context 取消等）。
func (o *JSONLFileObserver) OnError(err error, duration time.Duration) {
	o.write(map[string]any{
		"ts":          nowRFC3339(),
		"kind":        "error",
		"error":       err.Error(),
		"duration_ms": duration.Milliseconds(),
	})
}

// ensureOpen 在持有锁的前提下，确保文件已打开；首次调用时懒创建文件。
// 调用方必须已持有 o.mu。
func (o *JSONLFileObserver) ensureOpen() error {
	if o.f != nil {
		return nil // 已打开，直接复用
	}
	// 目录不存在则创建
	if err := os.MkdirAll(o.dir, 0o755); err != nil {
		return fmt.Errorf("创建 trace 目录失败: %w", err)
	}
	f, err := os.OpenFile(o.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开 trace 文件失败: %w", err)
	}
	o.f = f
	return nil
}

// write 串行写入一行 JSON。写失败时降级到 stderr，不阻塞业务。
func (o *JSONLFileObserver) write(record map[string]any) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// 懒创建：首次写入时才真正打开/创建文件
	if err := o.ensureOpen(); err != nil {
		fmt.Fprintf(os.Stderr, "[trace] 打开文件失败: %v\n", err)
		return
	}

	data, err := json.Marshal(record)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[trace] marshal 失败: %v\n", err)
		return
	}
	data = append(data, '\n')
	if _, err := o.f.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "[trace] 写入 %s 失败: %v\n", o.path, err)
	}
}

// 编译期断言。
var _ Observer = (*JSONLFileObserver)(nil)

// ------------------------------ 辅助函数 ------------------------------

// redactHeaders 把敏感头脱敏为 "***redacted***"，避免泄露 API Key。
// 返回 map[string][]string 方便 json.Marshal 输出数组形式。
func redactHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		lk := strings.ToLower(k)
		if lk == "authorization" || lk == "x-api-key" || lk == "api-key" {
			out[k] = []string{"***redacted***"}
			continue
		}
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// isJSON 粗略判断字节是否是 JSON 对象或数组，避免对非 JSON 响应调 RawMessage。
func isJSON(b []byte) bool {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return false
	}
	c := s[0]
	if c != '{' && c != '[' {
		return false
	}
	return json.Valid(b)
}

// ensureJSON 保证写入 payload 字段的是合法 JSON；不合法时返回 "null"，
// 避免 json.RawMessage 下游报错。
func ensureJSON(b []byte) []byte {
	if isJSON(b) {
		return b
	}
	return []byte("null")
}

// nowRFC3339 统一时间戳格式，便于排序与机器解析。
func nowRFC3339() string {
	return time.Now().Format(time.RFC3339Nano)
}