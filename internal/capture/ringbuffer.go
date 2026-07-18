package capture

import "sync"

type RingBuffer struct {
	mu       sync.RWMutex
	frames   [][]byte
	head     int
	count    int
	capacity int
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		frames:   make([][]byte, capacity),
		capacity: capacity,
	}
}

func (rb *RingBuffer) Push(frame []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.frames[rb.head] = frame
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
}

func (rb *RingBuffer) Latest() []byte {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.count == 0 {
		return nil
	}
	idx := (rb.head - 1 + rb.capacity) % rb.capacity
	return rb.frames[idx]
}

func (rb *RingBuffer) Snapshot() [][]byte {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	if rb.count == 0 {
		return nil
	}
	result := make([][]byte, rb.count)
	start := (rb.head - rb.count + rb.capacity) % rb.capacity
	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.capacity
		result[i] = rb.frames[idx]
	}
	return result
}
