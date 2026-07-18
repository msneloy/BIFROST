package capture

import "sync"

type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[*Subscriber]struct{}
	ringBuffer  *RingBuffer
}

type Subscriber struct {
	C    chan []byte
	done chan struct{}
}

func NewBroadcaster(ringBuffer *RingBuffer) *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[*Subscriber]struct{}),
		ringBuffer:  ringBuffer,
	}
}

func (b *Broadcaster) Subscribe() *Subscriber {
	s := &Subscriber{
		C:    make(chan []byte, 1),
		done: make(chan struct{}),
	}

	b.mu.Lock()
	b.subscribers[s] = struct{}{}
	b.mu.Unlock()

	// Send latest frame to new subscriber
	if latest := b.ringBuffer.Latest(); latest != nil {
		select {
		case s.C <- latest:
		default:
		}
	}

	return s
}

func (b *Broadcaster) Unsubscribe(s *Subscriber) {
	b.mu.Lock()
	delete(b.subscribers, s)
	b.mu.Unlock()
	close(s.done)
}

func (b *Broadcaster) RingBuffer() *RingBuffer {
	return b.ringBuffer
}

func (b *Broadcaster) Publish(frame []byte) {
	b.ringBuffer.Push(frame)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for s := range b.subscribers {
		select {
		case s.C <- frame:
		default:
			// Slow consumer — drop frame
		}
	}
}
