package stream

import "sync"

type Broadcaster struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan []byte]struct{}),
	}
}

func (b *Broadcaster) Subscribe(bufferSize int) chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan []byte, bufferSize)
	b.clients[ch] = struct{}{}
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	close(ch)
}

func (b *Broadcaster) Publish(data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- data:
		default:
		}
	}
}
