package tui

import (
	"testing"
	"time"

	"tinycode/internal/agent"
)

// TestChannelSinkEventsReadable 验证 Events() 返回可读的 channel。
func TestChannelSinkEventsReadable(t *testing.T) {
	sink := newChannelSink(4)
	defer sink.Close()

	ch := sink.Events()
	if ch == nil {
		t.Fatal("Events() returned nil channel")
	}

	// 新创建的 channel 应该是空的，读取会阻塞；用 select 验证它是可读 channel
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("new channel should be empty, but received an event")
		}
		// channel 已关闭，这是不期望的
		t.Fatal("new channel should not be closed")
	default:
		// 正确：channel 为空且未关闭
	}
}

// TestChannelSinkEmitAndReceive 验证 Emit 的事件可以从 Events() channel 读取到。
func TestChannelSinkEmitAndReceive(t *testing.T) {
	sink := newChannelSink(4)
	defer sink.Close()

	event := agent.Event{
		Kind:    agent.EventIterStart,
		Iter:    1,
		Payload: "test payload",
	}
	sink.Emit(event)

	select {
	case received := <-sink.Events():
		if received.Kind != event.Kind {
			t.Fatalf("expected Kind %q, got %q", event.Kind, received.Kind)
		}
		if received.Iter != event.Iter {
			t.Fatalf("expected Iter %d, got %d", event.Iter, received.Iter)
		}
		if received.Payload != event.Payload {
			t.Fatalf("expected Payload %q, got %q", event.Payload, received.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for emitted event")
	}
}

// TestChannelSinkEmitNonBlocking 验证 Emit 在 channel 满时不会阻塞，事件会被丢弃。
func TestChannelSinkEmitNonBlocking(t *testing.T) {
	// 使用缓冲区大小为 1 的 channel，便于制造"已满"场景
	sink := newChannelSink(1)
	defer sink.Close()

	// 填满 channel
	sink.Emit(agent.Event{Kind: agent.EventIterStart, Iter: 1})

	// 再次 Emit 不应阻塞（非阻塞投递）
	done := make(chan struct{})
	go func() {
		sink.Emit(agent.Event{Kind: agent.EventIterStart, Iter: 2})
		close(done)
	}()

	select {
	case <-done:
		// 成功：Emit 没有阻塞
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Emit blocked when channel is full")
	}

	// 验证 channel 中仍然只有第一个事件
	select {
	case evt := <-sink.Events():
		if evt.Iter != 1 {
			t.Fatalf("expected only event with Iter=1, got Iter=%d", evt.Iter)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the first event")
	}

	// channel 应该已经空了
	select {
	case <-sink.Events():
		t.Fatal("expected channel to be empty after reading the only event")
	default:
		// 正确：channel 为空
	}
}

// TestChannelSinkClose 验证 Close 后 Events() channel 被关闭。
func TestChannelSinkClose(t *testing.T) {
	sink := newChannelSink(4)

	// 先发送一些事件
	sink.Emit(agent.Event{Kind: agent.EventIterStart})
	sink.Emit(agent.Event{Kind: agent.EventAssistantReply})

	// 关闭 channel
	sink.Close()

	// 读取剩余事件
	count := 0
	for {
		_, ok := <-sink.Events()
		if !ok {
			break
		}
		count++
	}

	if count != 2 {
		t.Fatalf("expected 2 events before close, got %d", count)
	}

	// 再次读取应得到零值且 ok 为 false
	evt, ok := <-sink.Events()
	if ok {
		t.Fatalf("expected channel to be closed, but received %+v", evt)
	}
	// evt 应为零值：所有字段为空
	if evt.Kind != "" || evt.Iter != 0 || evt.Payload != "" {
		t.Fatalf("expected zero value from closed channel, got %+v", evt)
	}
}
