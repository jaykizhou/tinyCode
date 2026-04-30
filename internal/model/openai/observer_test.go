package openai_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tinycode/internal/model/openai"
)

// TestJSONLFileObserver_WritesRequestResponse 校验正常路径：
// 一次 Request + 一次 Response，产生两行合法 JSON；Authorization 头被脱敏。
func TestJSONLFileObserver_WritesRequestResponse(t *testing.T) {
	dir := t.TempDir()
	obs, err := openai.NewJSONLFileObserver(dir)
	if err != nil {
		t.Fatalf("NewJSONLFileObserver: %v", err)
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer sk-secret-abcdefg")

	obs.OnRequest("https://example.com/v1/chat/completions",
		headers, []byte(`{"model":"m","messages":[]}`))
	obs.OnResponse(200, []byte(`{"choices":[{"message":{"content":"hi"}}]}`), 50*time.Millisecond)
	if err := obs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 读取日志文件（目录里唯一一个文件）。
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("期望 1 个日志文件，got %d (err=%v)", len(entries), err)
	}
	path := filepath.Join(dir, entries[0].Name())
	if !strings.HasSuffix(path, ".jsonl") {
		t.Errorf("期望 .jsonl 扩展名, got %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var lines []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var m map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			t.Fatalf("每行应为合法 JSON: %v\n行内容: %s", err, scanner.Text())
		}
		lines = append(lines, m)
	}

	if len(lines) != 2 {
		t.Fatalf("期望 2 行记录, got %d", len(lines))
	}
	if lines[0]["kind"] != "request" || lines[1]["kind"] != "response" {
		t.Errorf("kind 顺序不对: %v / %v", lines[0]["kind"], lines[1]["kind"])
	}

	// 脱敏校验：Authorization 不应出现明文 token。
	hdrs, ok := lines[0]["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers 字段类型不对: %T", lines[0]["headers"])
	}
	auth := hdrs["Authorization"]
	if auth == nil {
		t.Fatal("Authorization 字段丢失")
	}
	if strings.Contains(toString(auth), "sk-secret") {
		t.Errorf("Authorization 未脱敏: %v", auth)
	}
}

// TestJSONLFileObserver_OnError 校验错误路径写出 kind=error 记录。
func TestJSONLFileObserver_OnError(t *testing.T) {
	dir := t.TempDir()
	obs, err := openai.NewJSONLFileObserver(dir)
	if err != nil {
		t.Fatalf("NewJSONLFileObserver: %v", err)
	}
	obs.OnError(errFake("dial tcp: i/o timeout"), 10*time.Millisecond)
	_ = obs.Close()

	data, err := os.ReadFile(obs.Path())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal(trim(data), &rec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rec["kind"] != "error" {
		t.Errorf("kind = %v, want error", rec["kind"])
	}
	if !strings.Contains(toString(rec["error"]), "timeout") {
		t.Errorf("error 字段不含预期文本: %v", rec["error"])
	}
}

// TestNopObserver 编译期保证接口与空实现无副作用。
func TestNopObserver(t *testing.T) {
	var o openai.Observer = openai.NopObserver{}
	o.OnRequest("x", nil, nil)
	o.OnResponse(200, nil, 0)
	o.OnError(errFake("x"), 0)
}

// --- helpers ---

type errFake string

func (e errFake) Error() string { return string(e) }

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		if len(x) == 0 {
			return ""
		}
		return toString(x[0])
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func trim(b []byte) []byte {
	return []byte(strings.TrimRight(string(b), "\n\r "))
}
