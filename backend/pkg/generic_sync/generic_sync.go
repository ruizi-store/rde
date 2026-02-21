// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package generic_sync 提供泛型同步原语
package generic_sync

import (
	"sync"
	"sync/atomic"
)

// Map 是一个泛型并发安全的 map
type Map[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewMap 创建一个新的泛型 Map
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		items: make(map[K]V),
	}
}

// Load 获取键对应的值
func (m *Map[K, V]) Load(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.items[key]
	return val, ok
}

// Store 存储键值对
func (m *Map[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = value
}

// Delete 删除键
func (m *Map[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// LoadOrStore 加载或存储值
func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if val, ok := m.items[key]; ok {
		return val, true
	}
	m.items[key] = value
	return value, false
}

// LoadAndDelete 加载并删除值
func (m *Map[K, V]) LoadAndDelete(key K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	val, ok := m.items[key]
	if ok {
		delete(m.items, key)
	}
	return val, ok
}

// Range 遍历所有键值对
func (m *Map[K, V]) Range(f func(key K, value V) bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.items {
		if !f(k, v) {
			break
		}
	}
}

// Len 返回元素数量
func (m *Map[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Clear 清空 map
func (m *Map[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[K]V)
}

// Keys 返回所有键
func (m *Map[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]K, 0, len(m.items))
	for k := range m.items {
		keys = append(keys, k)
	}
	return keys
}

// Values 返回所有值
func (m *Map[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	values := make([]V, 0, len(m.items))
	for _, v := range m.items {
		values = append(values, v)
	}
	return values
}

// Pool 是一个泛型对象池
type Pool[T any] struct {
	pool sync.Pool
	new  func() T
}

// NewPool 创建一个新的泛型 Pool
func NewPool[T any](newFunc func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return newFunc()
			},
		},
		new: newFunc,
	}
}

// Get 从池中获取对象
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put 将对象放回池中
func (p *Pool[T]) Put(x T) {
	p.pool.Put(x)
}

// Once 是一个泛型的 sync.Once，返回值
type Once[T any] struct {
	once sync.Once
	val  T
}

// Do 执行函数并返回结果
func (o *Once[T]) Do(f func() T) T {
	o.once.Do(func() {
		o.val = f()
	})
	return o.val
}

// Value 获取已计算的值
func (o *Once[T]) Value() T {
	return o.val
}

// Lazy 是一个惰性初始化的值
type Lazy[T any] struct {
	once sync.Once
	val  T
	init func() T
}

// NewLazy 创建一个惰性初始化的值
func NewLazy[T any](init func() T) *Lazy[T] {
	return &Lazy[T]{init: init}
}

// Get 获取值（如果未初始化则初始化）
func (l *Lazy[T]) Get() T {
	l.once.Do(func() {
		l.val = l.init()
	})
	return l.val
}

// Value 是一个泛型的原子值
type Value[T any] struct {
	v atomic.Value
}

// Load 原子加载值
func (v *Value[T]) Load() (T, bool) {
	val := v.v.Load()
	if val == nil {
		var zero T
		return zero, false
	}
	return val.(T), true
}

// Store 原子存储值
func (v *Value[T]) Store(val T) {
	v.v.Store(val)
}

// Swap 原子交换值
func (v *Value[T]) Swap(new T) (old T, loaded bool) {
	oldVal := v.v.Swap(new)
	if oldVal == nil {
		var zero T
		return zero, false
	}
	return oldVal.(T), true
}

// RWMutexMap 是带读写锁的泛型 map
type RWMutexMap[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewRWMutexMap 创建 RWMutexMap
func NewRWMutexMap[K comparable, V any]() *RWMutexMap[K, V] {
	return &RWMutexMap[K, V]{
		items: make(map[K]V),
	}
}

// Get 获取值
func (m *RWMutexMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.items[key]
	return v, ok
}

// Set 设置值
func (m *RWMutexMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = value
}

// Delete 删除值
func (m *RWMutexMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// GetOrSet 获取或设置值
func (m *RWMutexMap[K, V]) GetOrSet(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.items[key]; ok {
		return v, true
	}
	m.items[key] = value
	return value, false
}

// SetIfAbsent 如果不存在则设置
func (m *RWMutexMap[K, V]) SetIfAbsent(key K, value V) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[key]; ok {
		return false
	}
	m.items[key] = value
	return true
}

// Compute 计算并更新值
func (m *RWMutexMap[K, V]) Compute(key K, f func(V, bool) V) V {
	m.mu.Lock()
	defer m.mu.Unlock()
	old, ok := m.items[key]
	newVal := f(old, ok)
	m.items[key] = newVal
	return newVal
}

// ForEach 遍历所有键值对
func (m *RWMutexMap[K, V]) ForEach(f func(K, V)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for k, v := range m.items {
		f(k, v)
	}
}

// Size 返回大小
func (m *RWMutexMap[K, V]) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}
