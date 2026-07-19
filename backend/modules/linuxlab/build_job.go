package linuxlab

import (
	"sync"
	"sync/atomic"
	"time"
)

const defaultLogCapacity = 5000

// BuildLogEvent 带序号的构建日志（可供重连补齐）
type BuildLogEvent struct {
	Seq     int64  `json:"seq"`
	Status  string `json:"status,omitempty"`  // running|completed|failed
	Message string `json:"message,omitempty"`
	Line    string `json:"line,omitempty"`
	Done    bool   `json:"done,omitempty"` // 流结束标记（仅推送给订阅者）
}

type buildSub struct {
	ch   chan BuildLogEvent
	once sync.Once
}

func (s *buildSub) close() {
	s.once.Do(func() { close(s.ch) })
}

func (s *buildSub) send(ev BuildLogEvent) {
	select {
	case s.ch <- ev:
	default:
	}
}

// BuildJob 一次构建任务：环形日志缓冲 + 多路订阅
type BuildJob struct {
	ID        string
	Board     string
	Target    string
	StartedAt time.Time
	EndedAt   time.Time

	mu     sync.RWMutex
	status string // running|completed|failed
	next   int64  // 下一条序号（从 1 起）
	buf    []BuildLogEvent
	cap    int
	subs   map[*buildSub]struct{}
	done   atomic.Bool
}

func newBuildJob(board, target string, capacity int) *BuildJob {
	if capacity <= 0 {
		capacity = defaultLogCapacity
	}
	return &BuildJob{
		ID:        time.Now().UTC().Format("20060102T150405"),
		Board:     board,
		Target:    target,
		StartedAt: time.Now(),
		status:    "running",
		buf:       make([]BuildLogEvent, 0, capacity),
		cap:       capacity,
		subs:      make(map[*buildSub]struct{}),
	}
}

func (j *BuildJob) Status() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

func (j *BuildJob) LastSeq() int64 {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if j.next == 0 {
		return 0
	}
	return j.next - 1
}

func (j *BuildJob) IsDone() bool {
	return j.done.Load()
}

// AppendEvent 将 ProgressEvent 写入缓冲并广播
func (j *BuildJob) AppendEvent(ev ProgressEvent) {
	status := ev.Status
	if status == "" {
		status = "running"
	}

	if ev.Message != "" {
		j.append(BuildLogEvent{Status: status, Message: ev.Message})
	}
	if ev.Line != "" {
		j.append(BuildLogEvent{Status: "running", Line: ev.Line})
	}
	if status == "completed" || status == "failed" {
		if ev.Message == "" && ev.Line == "" {
			j.append(BuildLogEvent{Status: status, Message: status})
		}
		j.finish(status)
	}
}

func (j *BuildJob) append(ev BuildLogEvent) {
	j.mu.Lock()
	j.next++
	ev.Seq = j.next
	if len(j.buf) >= j.cap {
		copy(j.buf[0:], j.buf[1:])
		j.buf[len(j.buf)-1] = ev
		j.buf = j.buf[:j.cap]
	} else {
		j.buf = append(j.buf, ev)
	}
	subs := make([]*buildSub, 0, len(j.subs))
	for s := range j.subs {
		subs = append(subs, s)
	}
	j.mu.Unlock()

	for _, s := range subs {
		s.send(ev)
	}
}

func (j *BuildJob) finish(status string) {
	if !j.done.CompareAndSwap(false, true) {
		return
	}
	j.mu.Lock()
	j.status = status
	j.EndedAt = time.Now()
	subs := make([]*buildSub, 0, len(j.subs))
	for s := range j.subs {
		subs = append(subs, s)
	}
	j.subs = make(map[*buildSub]struct{})
	j.mu.Unlock()

	end := BuildLogEvent{Done: true, Status: status}
	for _, s := range subs {
		s.send(end)
		s.close()
	}
}

// SnapshotSince 返回 seq > since 的缓冲日志（按序）
func (j *BuildJob) SnapshotSince(since int64) []BuildLogEvent {
	j.mu.RLock()
	defer j.mu.RUnlock()
	out := make([]BuildLogEvent, 0, len(j.buf))
	for _, ev := range j.buf {
		if ev.Seq > since {
			out = append(out, ev)
		}
	}
	return out
}

// Subscribe 先推送 since 之后的历史，再接收实时；返回取消函数
func (j *BuildJob) Subscribe(since int64) (<-chan BuildLogEvent, func()) {
	sub := &buildSub{ch: make(chan BuildLogEvent, 512)}

	j.mu.Lock()
	history := make([]BuildLogEvent, 0, len(j.buf))
	for _, ev := range j.buf {
		if ev.Seq > since {
			history = append(history, ev)
		}
	}
	finished := j.done.Load()
	status := j.status
	if !finished {
		j.subs[sub] = struct{}{}
	}
	j.mu.Unlock()

	cancel := func() {
		j.mu.Lock()
		delete(j.subs, sub)
		j.mu.Unlock()
		sub.close()
	}

	go func() {
		for _, ev := range history {
			select {
			case sub.ch <- ev:
			case <-time.After(5 * time.Second):
				cancel()
				return
			}
		}
		if finished {
			select {
			case sub.ch <- BuildLogEvent{Done: true, Status: status}:
			default:
			}
			sub.close()
		}
	}()

	return sub.ch, cancel
}
