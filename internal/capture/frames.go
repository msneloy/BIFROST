package capture

import (
	"context"
	"io"
	"log"
)

const (
	jpegSOI = 0xFF // JPEG Start Of Image (first byte)
	jpegSOI2 = 0xD8 // JPEG Start Of Image (second byte)
	jpegEOI = 0xFF // JPEG End Of Image (first byte)
	jpegEOI2 = 0xD9 // JPEG End Of Image (second byte)
)

type FrameSplitter struct {
	reader      io.Reader
	broadcaster *Broadcaster
}

func NewFrameSplitter(r io.Reader, b *Broadcaster) *FrameSplitter {
	return &FrameSplitter{
		reader:      r,
		broadcaster: b,
	}
}

func (fs *FrameSplitter) Run(ctx context.Context) error {
	buf := make([]byte, 256*1024) // 256KB read buffer
	var leftover []byte

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, err := fs.reader.Read(buf)
		if n > 0 {
			data := append(leftover, buf[:n]...)
			leftover = fs.processFrames(data)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Printf("[!] FrameSplitter read error: %v", err)
			return err
		}
	}
}

func (fs *FrameSplitter) processFrames(data []byte) []byte {
	i := 0
	for i < len(data)-1 {
		// Scan for SOI marker
		if data[i] == jpegSOI && data[i+1] == jpegSOI2 {
			// Found SOI, scan for EOI
			start := i
			j := i + 2
			for j < len(data)-1 {
				if data[j] == jpegEOI && data[j+1] == jpegEOI2 {
					// Found complete frame
					frame := make([]byte, j+2-start)
					copy(frame, data[start:j+2])
					fs.broadcaster.Publish(frame)
					i = j + 2
					goto next
				}
				j++
			}
			// EOI not found in this chunk — return remaining as leftover
			return data[start:]
		}
		i++
		continue
	next:
	}

	// Return any trailing bytes that might contain a partial frame
	if i < len(data) {
		return data[i:]
	}
	return nil
}
