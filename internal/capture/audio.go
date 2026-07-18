package capture

import (
	"sync"
)

type AudioBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[*AudioSubscriber]struct{}
	latest      []byte
}

type AudioSubscriber struct {
	C    chan []byte
	done chan struct{}
}

func NewAudioBroadcaster() *AudioBroadcaster {
	return &AudioBroadcaster{
		subscribers: make(map[*AudioSubscriber]struct{}),
	}
}

func (ab *AudioBroadcaster) Subscribe() *AudioSubscriber {
	s := &AudioSubscriber{
		C:    make(chan []byte, 64),
		done: make(chan struct{}),
	}

	ab.mu.Lock()
	ab.subscribers[s] = struct{}{}
	ab.mu.Unlock()

	return s
}

func (ab *AudioBroadcaster) Unsubscribe(s *AudioSubscriber) {
	ab.mu.Lock()
	delete(ab.subscribers, s)
	ab.mu.Unlock()
	close(s.done)
}

func (ab *AudioBroadcaster) Publish(chunk []byte) {
	ab.mu.Lock()
	ab.latest = chunk
	ab.mu.Unlock()

	ab.mu.RLock()
	defer ab.mu.RUnlock()
	for s := range ab.subscribers {
		select {
		case s.C <- chunk:
		default:
			// Slow consumer — drop audio chunk
		}
	}
}
