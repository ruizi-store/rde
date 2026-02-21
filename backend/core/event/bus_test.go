package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	var received Event
	bus.Subscribe("test.event", func(e Event) {
		received = e
	})

	bus.PublishFrom("test-module", "test.event", map[string]string{"key": "value"})

	assert.Equal(t, "test.event", received.Type)
	assert.Equal(t, "test-module", received.Source)
	assert.Equal(t, map[string]string{"key": "value"}, received.Data)
	assert.Greater(t, received.Timestamp, int64(0))
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	var count int32
	for i := 0; i < 3; i++ {
		bus.Subscribe("test.event", func(e Event) {
			atomic.AddInt32(&count, 1)
		})
	}

	bus.Publish("test.event", nil)

	assert.Equal(t, int32(3), atomic.LoadInt32(&count))
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	var count int32
	unsubscribe := bus.Subscribe("test.event", func(e Event) {
		atomic.AddInt32(&count, 1)
	})

	bus.Publish("test.event", nil)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count))

	unsubscribe()

	bus.Publish("test.event", nil)
	assert.Equal(t, int32(1), atomic.LoadInt32(&count)) // 不应增加
}

func TestBus_WildcardSubscription(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	var events []string
	var mu sync.Mutex

	bus.SubscribeAll(func(e Event) {
		mu.Lock()
		events = append(events, e.Type)
		mu.Unlock()
	})

	bus.Publish("event.a", nil)
	bus.Publish("event.b", nil)
	bus.Publish("event.c", nil)

	assert.Len(t, events, 3)
	assert.Contains(t, events, "event.a")
	assert.Contains(t, events, "event.b")
	assert.Contains(t, events, "event.c")
}

func TestBus_AsyncPublish(t *testing.T) {
	bus := NewBus(zap.NewNop()) // 异步模式

	var wg sync.WaitGroup
	wg.Add(1)

	var received bool
	bus.Subscribe("test.event", func(e Event) {
		received = true
		wg.Done()
	})

	bus.Publish("test.event", nil)

	// 等待异步处理完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		assert.True(t, received)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for async handler")
	}
}

func TestBus_HandlerPanicRecovery(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	var secondHandlerCalled bool

	bus.Subscribe("test.event", func(e Event) {
		panic("intentional panic")
	})

	bus.Subscribe("test.event", func(e Event) {
		secondHandlerCalled = true
	})

	// 不应 panic
	require.NotPanics(t, func() {
		bus.Publish("test.event", nil)
	})

	// 第二个处理器应该仍然被调用
	assert.True(t, secondHandlerCalled)
}

func TestBus_HandlerCount(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	assert.Equal(t, 0, bus.HandlerCount("test.event"))

	bus.Subscribe("test.event", func(e Event) {})
	assert.Equal(t, 1, bus.HandlerCount("test.event"))

	bus.Subscribe("test.event", func(e Event) {})
	assert.Equal(t, 2, bus.HandlerCount("test.event"))

	bus.Subscribe("other.event", func(e Event) {})
	assert.Equal(t, 2, bus.HandlerCount("test.event"))
	assert.Equal(t, 1, bus.HandlerCount("other.event"))
}

func TestBus_Clear(t *testing.T) {
	bus := NewSyncBus(zap.NewNop())

	bus.Subscribe("test.event", func(e Event) {})
	bus.Subscribe("other.event", func(e Event) {})

	assert.Equal(t, 1, bus.HandlerCount("test.event"))
	assert.Equal(t, 1, bus.HandlerCount("other.event"))

	bus.Clear()

	assert.Equal(t, 0, bus.HandlerCount("test.event"))
	assert.Equal(t, 0, bus.HandlerCount("other.event"))
}
