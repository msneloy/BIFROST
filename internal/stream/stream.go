package stream

import (
	"sync"
)

type Broadcaster struct {
	mu        sync.RWMutex
	clients   map[chan []byte]struct{}
	header    []byte
	Total     int64
	PubRate   int64 // Published bytes in last cycle
	lastFrame []byte
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan []byte]struct{}),
	}
}

func (b *Broadcaster) SetHeader(header []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if header == nil {
		b.header = nil
		return
	}
	b.header = make([]byte, len(header))
	copy(b.header, header)
}

func (b *Broadcaster) GetHeader() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.header
}

func (b *Broadcaster) Subscribe(bufferSize int) chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan []byte, bufferSize)

	// If we have a header (e.g. MJPEG boundary or fMP4 moov), send it first
	if len(b.header) > 0 {
		select {
		case ch <- b.header:
		default:
		}
	}

	b.clients[ch] = struct{}{}
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	// We don't close(ch) here to avoid panics in Publish
}

func (b *Broadcaster) Publish(data []byte) {
	if len(data) == 0 {
		return
	}

	b.mu.Lock()
	b.Total += int64(len(data))
	b.PubRate += int64(len(data))
	b.lastFrame = make([]byte, len(data))
	copy(b.lastFrame, data)
	b.mu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()

	for ch := range b.clients {
		select {
		case ch <- data:
		default:
			// Buffer full, skip this frame for this client
		}
	}
}

func (b *Broadcaster) GetLastFrame() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.lastFrame == nil {
		return nil
	}
	res := make([]byte, len(b.lastFrame))
	copy(res, b.lastFrame)
	return res
}

func (b *Broadcaster) GetPubRate() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	rate := b.PubRate
	b.PubRate = 0
	return rate
}
