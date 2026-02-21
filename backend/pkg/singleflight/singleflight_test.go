package singleflight

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroup_Do(t *testing.T) {
	var g Group
	counter := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		return "result", nil
	}

	v, err, _ := g.Do("key", fn)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if v != "result" {
		t.Errorf("Expected 'result', got %v", v)
	}
	if counter != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}
}

func TestGroup_Do_DuplicateSuppression(t *testing.T) {
	var g Group
	counter := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err, shared := g.Do("key", fn)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if v != "result" {
				t.Errorf("Expected 'result', got %v", v)
			}
			_ = shared
		}()
	}
	wg.Wait()

	// 只应执行一次
	if counter != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}
}

func TestGroup_Do_Error(t *testing.T) {
	var g Group
	expectedErr := errors.New("test error")

	fn := func() (interface{}, error) {
		return nil, expectedErr
	}

	_, err, _ := g.Do("key", fn)
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

func TestGroup_DoChan(t *testing.T) {
	var g Group
	counter := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(10 * time.Millisecond)
		return "result", nil
	}

	ch := g.DoChan("key", fn)
	result := <-ch

	if result.Err != nil {
		t.Errorf("Unexpected error: %v", result.Err)
	}
	if result.Val != "result" {
		t.Errorf("Expected 'result', got %v", result.Val)
	}
}

func TestGroup_DoChan_DuplicateSuppression(t *testing.T) {
	var g Group
	counter := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := g.DoChan("key", fn)
			result := <-ch
			if result.Err != nil {
				t.Errorf("Unexpected error: %v", result.Err)
			}
		}()
	}
	wg.Wait()

	if counter != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}
}

func TestGroup_Forget(t *testing.T) {
	var g Group
	counter := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		return "result", nil
	}

	// 第一次调用
	g.Do("key", fn)
	if counter != 1 {
		t.Errorf("Expected counter to be 1, got %d", counter)
	}

	// Forget 后再次调用
	g.Forget("key")
	g.Do("key", fn)
	if counter != 2 {
		t.Errorf("Expected counter to be 2, got %d", counter)
	}
}

func TestGroup_Do_DifferentKeys(t *testing.T) {
	var g Group
	counter := int32(0)

	fn := func() (interface{}, error) {
		atomic.AddInt32(&counter, 1)
		return "result", nil
	}

	g.Do("key1", fn)
	g.Do("key2", fn)
	g.Do("key3", fn)

	if counter != 3 {
		t.Errorf("Expected counter to be 3, got %d", counter)
	}
}
