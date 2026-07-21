package capture

import (
	"sync"
)

const (
	EntryTypeVideo byte = 0x01
	EntryTypeAudio byte = 0x02
)

type MuxEntry struct {
	Type byte
	Data []byte
}

type MuxBuffer struct {
	mu       sync.RWMutex
	entries  []MuxEntry
	head     int
	count    int
	capacity int

	subMu sync.RWMutex
	subs  map[*MuxSubscriber]struct{}
}

type MuxSubscriber struct {
	C    chan []byte
	done chan struct{}
}

func NewMuxBuffer(capacity int) *MuxBuffer {
	return &MuxBuffer{
		entries:  make([]MuxEntry, capacity),
		capacity: capacity,
		subs:     make(map[*MuxSubscriber]struct{}),
	}
}

func (mb *MuxBuffer) push(entry MuxEntry) {
	mb.mu.Lock()
	mb.entries[mb.head] = entry
	mb.head = (mb.head + 1) % mb.capacity
	if mb.count < mb.capacity {
		mb.count++
	}
	mb.mu.Unlock()

	mb.subMu.RLock()
	defer mb.subMu.RUnlock()

	if len(mb.subs) == 0 {
		return
	}

	// Marshal once into a temp buffer, then copy to each subscriber
	data := entry.Marshal()

	for s := range mb.subs {
		chunk := make([]byte, len(data))
		copy(chunk, data)
		select {
		case s.C <- chunk:
		default:
		}
	}
}

func (mb *MuxBuffer) PublishVideo(frame []byte) {
	mb.push(MuxEntry{Type: EntryTypeVideo, Data: frame})
}

func (mb *MuxBuffer) PublishAudio(chunk []byte) {
	mb.push(MuxEntry{Type: EntryTypeAudio, Data: chunk})
}

func (mb *MuxBuffer) Subscribe() *MuxSubscriber {
	s := &MuxSubscriber{
		C:    make(chan []byte, 128),
		done: make(chan struct{}),
	}

	mb.subMu.Lock()
	mb.subs[s] = struct{}{}
	mb.subMu.Unlock()

	// Replay only the latest video frame for late-joiners
	mb.mu.RLock()
	if mb.count > 0 {
		for i := mb.count - 1; i >= 0; i-- {
			idx := (mb.head - 1 - (mb.count - 1 - i) + mb.capacity) % mb.capacity
			if mb.entries[idx].Type == EntryTypeVideo {
				data := mb.entries[idx].Marshal()
				select {
				case s.C <- data:
				default:
				}
				break
			}
		}
	}
	mb.mu.RUnlock()

	return s
}

func (mb *MuxBuffer) Unsubscribe(s *MuxSubscriber) {
	mb.subMu.Lock()
	delete(mb.subs, s)
	mb.subMu.Unlock()
	close(s.done)
}

func (mb *MuxBuffer) LatestVideo() []byte {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	if mb.count == 0 {
		return nil
	}
	for i := mb.count - 1; i >= 0; i-- {
		idx := (mb.head - 1 - (mb.count - 1 - i) + mb.capacity) % mb.capacity
		if mb.entries[idx].Type == EntryTypeVideo {
			return mb.entries[idx].Data
		}
	}
	return nil
}

// Marshal writes [type(1)][length(4)][data(N)] into a new byte slice.
func (e MuxEntry) Marshal() []byte {
	dataLen := len(e.Data)
	buf := make([]byte, 5+dataLen)
	buf[0] = e.Type
	buf[1] = byte(dataLen >> 24)
	buf[2] = byte(dataLen >> 16)
	buf[3] = byte(dataLen >> 8)
	buf[4] = byte(dataLen)
	copy(buf[5:], e.Data)
	return buf
}
