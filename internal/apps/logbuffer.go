package apps

import "sync"

type LogBuffer struct {
	mu      sync.RWMutex
	lines   []string
	maxSize int

	subMu sync.Mutex
	subs  map[chan string]struct{}
}

func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		maxSize: maxSize,
		subs:    make(map[chan string]struct{}),
	}
}

func (lb *LogBuffer) Append(line string) {
	lb.mu.Lock()
	lb.lines = append(lb.lines, line)
	if len(lb.lines) > lb.maxSize {
		lb.lines = lb.lines[len(lb.lines)-lb.maxSize:]
	}
	lb.mu.Unlock()

	lb.subMu.Lock()
	for ch := range lb.subs {
		select {
		case ch <- line:
		default:
		}
	}
	lb.subMu.Unlock()
}

func (lb *LogBuffer) Reset() {
	lb.mu.Lock()
	lb.lines = nil
	lb.mu.Unlock()
}

func (lb *LogBuffer) Recent(n int) []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	if n > len(lb.lines) {
		n = len(lb.lines)
	}
	out := make([]string, n)
	copy(out, lb.lines[len(lb.lines)-n:])
	return out
}

func (lb *LogBuffer) Subscribe() chan string {
	ch := make(chan string, 64)
	lb.subMu.Lock()
	lb.subs[ch] = struct{}{}
	lb.subMu.Unlock()
	return ch
}

func (lb *LogBuffer) Unsubscribe(ch chan string) {
	lb.subMu.Lock()
	delete(lb.subs, ch)
	lb.subMu.Unlock()
	close(ch)
}
