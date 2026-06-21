package stream

import (
	"sync"
)

// Broadcaster is a thread-safe pub/sub hub that fans out byte slices to all
// registered subscribers. Used for both MJPEG video frames and MP3 audio chunks.
type Broadcaster struct {
	mu        sync.RWMutex
	clients   map[chan []byte]struct{}
	header    []byte
	Total     int64
	PubRate   int64 // Published bytes in last cycle
	lastFrame []byte
}

// NewBroadcaster creates a new Broadcaster with an empty subscriber set.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan []byte]struct{}),
	}
}

// SetHeader sets optional metadata sent to new subscribers on connect.
// A nil header clears the current value. A header of "BRIDGE" signals the
// capture goroutine to stop binary capture in favor of browser-native push.
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

// GetHeader returns the current header metadata.
func (b *Broadcaster) GetHeader() []byte {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.header
}

// Subscribe creates a buffered channel of the given size, registers it as a
// subscriber, and immediately sends the current header if one is set.
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

// Unsubscribe removes a channel from the subscriber set. The channel is not
// closed to avoid panics in concurrent Publish calls.
func (b *Broadcaster) Unsubscribe(ch chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	// We don't close(ch) here to avoid panics in Publish
}

// Publish sends data to all subscribers. Non-blocking: if a subscriber's
// buffer is full the frame is dropped for that client. Also updates Total,
// PubRate, and stores a copy as lastFrame for single-frame requests.
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

// GetLastFrame returns a copy of the most recently published frame, or nil
// if no frames have been published yet. Used by the /frame endpoint.
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

// GetPubRate returns the number of bytes published since the last call and
// resets the counter. Used by the dashboard for bandwidth display.
func (b *Broadcaster) GetPubRate() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	rate := b.PubRate
	b.PubRate = 0
	return rate
}
