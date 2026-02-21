package generic_sync

import (
	"sync"
	"testing"
)

func TestMap_BasicOperations(t *testing.T) {
	m := NewMap[string, int]()

	// Test Store and Load
	m.Store("key1", 100)
	val, ok := m.Load("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	if val != 100 {
		t.Errorf("Expected 100, got %d", val)
	}

	// Test non-existent key
	_, ok = m.Load("nonexistent")
	if ok {
		t.Error("Expected nonexistent key to not exist")
	}
}

func TestMap_Delete(t *testing.T) {
	m := NewMap[string, string]()
	m.Store("key1", "value1")
	m.Delete("key1")

	_, ok := m.Load("key1")
	if ok {
		t.Error("Expected key1 to be deleted")
	}
}

func TestMap_LoadOrStore(t *testing.T) {
	m := NewMap[string, int]()

	// Store new value
	val, loaded := m.LoadOrStore("key1", 100)
	if loaded {
		t.Error("Expected loaded to be false for new key")
	}
	if val != 100 {
		t.Errorf("Expected 100, got %d", val)
	}

	// Load existing value
	val, loaded = m.LoadOrStore("key1", 200)
	if !loaded {
		t.Error("Expected loaded to be true for existing key")
	}
	if val != 100 {
		t.Errorf("Expected 100, got %d", val)
	}
}

func TestMap_LoadAndDelete(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("key1", 100)

	val, ok := m.LoadAndDelete("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	if val != 100 {
		t.Errorf("Expected 100, got %d", val)
	}

	_, ok = m.Load("key1")
	if ok {
		t.Error("Expected key1 to be deleted")
	}
}

func TestMap_Range(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	m.Store("c", 3)

	count := 0
	m.Range(func(key string, value int) bool {
		count++
		return true
	})

	if count != 3 {
		t.Errorf("Expected 3 iterations, got %d", count)
	}
}

func TestMap_Len(t *testing.T) {
	m := NewMap[int, string]()
	if m.Len() != 0 {
		t.Error("Expected empty map")
	}

	m.Store(1, "one")
	m.Store(2, "two")
	if m.Len() != 2 {
		t.Errorf("Expected length 2, got %d", m.Len())
	}
}

func TestMap_Clear(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)
	m.Clear()

	if m.Len() != 0 {
		t.Error("Expected empty map after clear")
	}
}

func TestMap_Keys(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)

	keys := m.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestMap_Values(t *testing.T) {
	m := NewMap[string, int]()
	m.Store("a", 1)
	m.Store("b", 2)

	values := m.Values()
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
}

func TestMap_Concurrent(t *testing.T) {
	m := NewMap[int, int]()
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.Store(n, n*10)
		}(i)
	}
	wg.Wait()

	if m.Len() != 100 {
		t.Errorf("Expected 100 entries, got %d", m.Len())
	}
}

func TestPool(t *testing.T) {
	counter := 0
	p := NewPool(func() int {
		counter++
		return counter
	})

	// Get should create new value
	v1 := p.Get()
	if v1 != 1 {
		t.Errorf("Expected 1, got %d", v1)
	}

	// Put and Get again
	p.Put(v1)
	v2 := p.Get()
	// v2 might be 1 (reused) or 2 (new), both are valid
	if v2 != 1 && v2 != 2 {
		t.Errorf("Expected 1 or 2, got %d", v2)
	}
}

func TestOnce(t *testing.T) {
	var o Once[int]
	counter := 0

	result := o.Do(func() int {
		counter++
		return 42
	})

	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	// Second call should return same value without executing
	result = o.Do(func() int {
		counter++
		return 100
	})

	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	if counter != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}
}

func TestLazy(t *testing.T) {
	counter := 0
	lazy := NewLazy(func() string {
		counter++
		return "initialized"
	})

	// First call should initialize
	v1 := lazy.Get()
	if v1 != "initialized" {
		t.Errorf("Expected 'initialized', got %s", v1)
	}

	// Second call should not reinitialize
	v2 := lazy.Get()
	if v2 != "initialized" {
		t.Errorf("Expected 'initialized', got %s", v2)
	}

	if counter != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}
}

func TestValue(t *testing.T) {
	var v Value[string]

	// Initially empty
	_, ok := v.Load()
	if ok {
		t.Error("Expected empty value")
	}

	// Store and Load
	v.Store("hello")
	val, ok := v.Load()
	if !ok {
		t.Error("Expected value to exist")
	}
	if val != "hello" {
		t.Errorf("Expected 'hello', got %s", val)
	}
}
