package tui

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestIndent 验证 indent 给多行文本整体缩进 n 个空格。
func TestIndent(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{
			name: "empty string",
			s:    "",
			n:    4,
			want: "",
		},
		{
			name: "single line",
			s:    "hello",
			n:    2,
			want: "  hello",
		},
		{
			name: "multiple lines",
			s:    "line1\nline2\nline3",
			n:    4,
			want: "    line1\n    line2\n    line3",
		},
		{
			name: "zero indent",
			s:    "hello",
			n:    0,
			want: "hello",
		},
		{
			name: "trailing newline",
			s:    "line1\nline2\n",
			n:    2,
			want: "  line1\n  line2\n  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indent(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("indent(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}

// TestTruncateLong 验证 truncateLong 对超长字符串的头尾保留截断。
func TestTruncateLong(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "zero max",
			s:    "hello",
			max:  0,
			want: "hello",
		},
		{
			name: "negative max",
			s:    "hello",
			max:  -1,
			want: "hello",
		},
		{
			name: "exact length",
			s:    "hello",
			max:  5,
			want: "hello",
		},
		{
			name: "longer than max (even)",
			s:    "abcdefghij",
			max:  6,
			want: "abc\n\n... [已截断] ...\n\nhij",
		},
		{
			name: "longer than max (odd)",
			s:    "abcdefghijk",
			max:  7,
			want: "abc\n\n... [已截断] ...\n\nijk",
		},
		{
			name: "short string",
			s:    "hi",
			max:  10,
			want: "hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateLong(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateLong(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

// TestShortPath 验证 shortPath 把过长路径缩短为末尾两级。
func TestShortPath(t *testing.T) {
	tests := []struct {
		name string
		p    string
		want string
	}{
		{
			name: "short path",
			p:    "/home/user",
			want: "/home/user",
		},
		{
			name: "exactly 40 chars",
			p:    strings.Repeat("a", 40),
			want: strings.Repeat("a", 40),
		},
		{
			name: "long path with many parts",
			p:    "/Users/jayki/workspace/go_project/tinyCode/internal/ui/tui",
			want: ".../ui/tui",
		},
		{
			name: "very long single name path",
			p:    "/a/" + strings.Repeat("b", 50),
			want: ".../a/" + strings.Repeat("b", 50),
		},
		{
			name: "two parts long",
			p:    "/" + strings.Repeat("x", 25) + "/" + strings.Repeat("y", 25),
			want: ".../" + strings.Repeat("x", 25) + "/" + strings.Repeat("y", 25),
		},
		{
			name: "empty path",
			p:    "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortPath(tt.p)
			if got != tt.want {
				t.Errorf("shortPath(%q) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

// TestMaxToolResult 验证 maxToolResult 随宽度动态调节。
func TestMaxToolResult(t *testing.T) {
	tests := []struct {
		name  string
		width int
		want  int
	}{
		{
			name:  "positive width",
			width: 100,
			want:  6000,
		},
		{
			name:  "zero width falls back",
			width: 0,
			want:  6000,
		},
		{
			name:  "negative width falls back",
			width: -10,
			want:  6000,
		},
		{
			name:  "typical terminal width",
			width: 120,
			want:  7200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxToolResult(tt.width)
			if got != tt.want {
				t.Errorf("maxToolResult(%d) = %d, want %d", tt.width, got, tt.want)
			}
		})
	}
}

// TestArgsSummary 验证 argsSummary 把 JSON 参数压缩成一行显示。
func TestArgsSummary(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{
			name: "empty raw message",
			raw:  nil,
			want: "{}",
		},
		{
			name: "empty bytes",
			raw:  json.RawMessage{},
			want: "{}",
		},
		{
			name: "valid compact json",
			raw:  json.RawMessage(`{"name":"test","value":42}`),
			want: `{"name":"test","value":42}`,
		},
		{
			name: "valid pretty json",
			raw:  json.RawMessage("{\n  \"name\": \"test\",\n  \"value\": 42\n}"),
			want: `{"name":"test","value":42}`,
		},
		{
			name: "nested object",
			raw:  json.RawMessage(`{"outer":{"inner":true}}`),
			want: `{"outer":{"inner":true}}`,
		},
		{
			name: "invalid json fallback",
			raw:  json.RawMessage(`{invalid}`),
			want: `{invalid}`,
		},
		{
			name: "array input",
			raw:  json.RawMessage(`[1,2,3]`),
			want: `[1,2,3]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := argsSummary(tt.raw)
			if got != tt.want {
				t.Errorf("argsSummary(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
