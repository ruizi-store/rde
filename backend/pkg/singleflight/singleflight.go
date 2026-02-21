// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package singleflight 提供重复函数调用抑制机制
package singleflight

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
)

// ErrForgotten 表示调用被忘记
var ErrForgotten = errors.New("call was forgotten")

// call 表示一个正在进行的或已完成的调用
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error

	// 调用者数量
	dups  int
	chans []chan<- Result
}

// Group 表示一类工作的 singleflight 组
type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

// Result 保存 Do 的结果
type Result struct {
	Val    interface{}
	Err    error
	Shared bool
}

// Do 执行给定函数并返回结果，确保同一时间
// 对于同一个 key 只有一个执行在进行中。
// 如果有重复调用，调用者会等待原始调用完成并收到相同的结果。
func (g *Group) Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	g.doCall(c, key, fn)
	return c.val, c.err, c.dups > 0
}

// DoChan 类似 Do，但返回一个 channel，当结果就绪时会接收到值
func (g *Group) DoChan(key string, fn func() (interface{}, error)) <-chan Result {
	ch := make(chan Result, 1)
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		c.chans = append(c.chans, ch)
		g.mu.Unlock()
		return ch
	}
	c := &call{chans: []chan<- Result{ch}}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	go g.doCall(c, key, fn)

	return ch
}

// doCall 处理单个键的函数调用
func (g *Group) doCall(c *call, key string, fn func() (interface{}, error)) {
	normalReturn := false
	recovered := false

	// 使用双重 defer 来区分 panic 和 runtime.Goexit
	defer func() {
		if !normalReturn && !recovered {
			c.err = ErrForgotten
		}

		g.mu.Lock()
		defer g.mu.Unlock()
		c.wg.Done()
		if g.m[key] == c {
			delete(g.m, key)
		}

		if e, ok := c.err.(*panicError); ok {
			// 对于 panic，在等待的 goroutine 中也触发 panic
			if len(c.chans) > 0 {
				go panic(e)
				select {} // 保持这个 goroutine 存活
			} else {
				panic(e)
			}
		} else if c.err == ErrForgotten {
			// runtime.Goexit 情况
		}

		for _, ch := range c.chans {
			ch <- Result{c.val, c.err, c.dups > 0}
		}
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				c.err = newPanicError(r)
			}
		}()

		c.val, c.err = fn()
		normalReturn = true
	}()

	if !normalReturn {
		recovered = true
	}
}

// Forget 告诉 singleflight 忘记一个 key，
// 之后的同 key 调用将执行函数而不是等待之前的调用完成
func (g *Group) Forget(key string) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}

// panicError 用于将 panic 值包装成 error
type panicError struct {
	value interface{}
	stack []byte
}

func (p *panicError) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

func (p *panicError) Unwrap() error {
	err, ok := p.value.(error)
	if !ok {
		return nil
	}
	return err
}

func newPanicError(v interface{}) error {
	stack := debug.Stack()

	// 移除前几行堆栈信息
	if line := bytes.IndexByte(stack, '\n'); line >= 0 {
		stack = stack[line+1:]
	}

	return &panicError{value: v, stack: stack}
}

// DoContext 带有 context 支持的 Do
func (g *Group) DoContext(ctx context.Context, key string, fn func(ctx context.Context) (interface{}, error)) (v interface{}, err error, shared bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		g.mu.Unlock()
		
		// 等待完成或 context 取消
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			return c.val, c.err, true
		case <-ctx.Done():
			return nil, ctx.Err(), true
		}
	}
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	g.doCallContext(ctx, c, key, fn)
	return c.val, c.err, c.dups > 0
}

// doCallContext 处理带 context 的函数调用
func (g *Group) doCallContext(ctx context.Context, c *call, key string, fn func(ctx context.Context) (interface{}, error)) {
	defer func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		c.wg.Done()
		if g.m[key] == c {
			delete(g.m, key)
		}
	}()

	c.val, c.err = fn(ctx)
}

// GenericGroup 泛型版本的 singleflight
type GenericGroup[T any] struct {
	mu sync.Mutex
	m  map[string]*genericCall[T]
}

type genericCall[T any] struct {
	wg   sync.WaitGroup
	val  T
	err  error
	dups int
}

// GenericResult 泛型结果
type GenericResult[T any] struct {
	Val    T
	Err    error
	Shared bool
}

// NewGenericGroup 创建泛型 singleflight
func NewGenericGroup[T any]() *GenericGroup[T] {
	return &GenericGroup[T]{
		m: make(map[string]*genericCall[T]),
	}
}

// Do 执行并返回结果
func (g *GenericGroup[T]) Do(key string, fn func() (T, error)) (T, error, bool) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*genericCall[T])
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err, true
	}
	c := new(genericCall[T])
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	defer func() {
		g.mu.Lock()
		c.wg.Done()
		delete(g.m, key)
		g.mu.Unlock()
	}()

	c.val, c.err = fn()
	return c.val, c.err, c.dups > 0
}

// Forget 忘记一个 key
func (g *GenericGroup[T]) Forget(key string) {
	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()
}
