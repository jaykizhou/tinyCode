package agent

import "sync"

// Conversation 管理一次会话的消息历史。
//
// 核心不变量（对应 one_loop.md 5.3 的"铁律"）：
//
//	messages 数组只追加（append），永远不修改已有元素。
//
// 这样能保证：
//  1. 模型上下文一致性：任何一次调用看到的历史都与上一次一致；
//  2. 可审计性：整个执行流程就是一条只增不减的日志；
//  3. 简化并发：读者拿到的快照天然是不可变的（因为我们返回副本）。
type Conversation struct {
	mu       sync.RWMutex
	messages []Message
}

// NewConversation 构造一个空会话。
func NewConversation() *Conversation {
	return &Conversation{}
}

// Append 在会话末尾追加一条消息，返回新消息总数。
// 该方法是整个类型唯一允许"改变内部状态"的入口。
func (c *Conversation) Append(m Message) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, m)
	return len(c.messages)
}

// Snapshot 返回当前消息的只读副本。
//
// 之所以返回副本而不是底层切片，是为了防止调用方无意中通过索引修改了历史消息，
// 从而破坏"只追加不修改"的不变量。
func (c *Conversation) Snapshot() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Message, len(c.messages))
	copy(out, c.messages)
	return out
}

// Len 返回当前消息数量。
func (c *Conversation) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.messages)
}
