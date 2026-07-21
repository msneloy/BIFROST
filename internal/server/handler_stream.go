package server

import (
	"log"
	"net/http"

	"github.com/nelobster/bifrost/internal/capture"
)

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if guardCheck(r, w, s.tracker) {
		return
	}

	ip := extractIP(r)
	s.tracker.GetOrCreate(ip)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.capture.MuxBuffer().Subscribe()
	defer s.capture.MuxBuffer().Unsubscribe(sub)

	log.Printf("[+] Stream client connected: %s", ip)

	// Batch buffer — accumulates multiple entries before a single write+flush
	var batch []byte
	var hasVideo bool

	for {
		select {
		case <-r.Context().Done():
			log.Printf("[-] Stream client disconnected: %s", ip)
			return
		case data, ok := <-sub.C:
			if !ok {
				return
			}
			batch = append(batch, data...)
			if data[0] == capture.EntryTypeVideo {
				hasVideo = true
			}

			// Drain any more pending entries without blocking
		drain:
			for {
				select {
				case extra, ok := <-sub.C:
					if !ok {
						break drain
					}
					batch = append(batch, extra...)
					if extra[0] == capture.EntryTypeVideo {
						hasVideo = true
					}
				default:
					break drain
				}
			}

			// Single write for the whole batch
			if _, err := w.Write(batch); err != nil {
				return
			}
			s.tracker.AddBytes(ip, uint64(len(batch)))

			// Flush only when we have at least one video frame
			if hasVideo {
				flusher.Flush()
			}
			batch = batch[:0]
			hasVideo = false
		}
	}
}
