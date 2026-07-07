package webrtc

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"
)

// SFU is a Selective Forwarding Unit that broadcasts media to all connected peers.
type SFU struct {
	mu          sync.RWMutex
	mediaEngine *webrtc.MediaEngine
	api         *webrtc.API
	peers       map[string]*webrtc.PeerConnection
	videoTrack  *webrtc.TrackLocalStaticRTP
	audioTrack  *webrtc.TrackLocalStaticRTP
	onPeerJoin  func(id string)
	onPeerLeave func(id string)
}

// NewSFU creates a new SFU with VP8 video and Opus audio codecs.
func NewSFU() *SFU {
	mediaEngine := &webrtc.MediaEngine{}

	// Register VP8 for video
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeVP8,
			ClockRate: 90000,
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		log.Fatalf("Failed to register VP8: %v", err)
	}

	// Register Opus for audio
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		log.Fatalf("Failed to register Opus: %v", err)
	}

	ir := &interceptor.Registry{}
	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(ir),
	)

	videoTrack, _ := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8},
		"video", "bifrost",
	)

	audioTrack, _ := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "bifrost",
	)

	return &SFU{
		mediaEngine: mediaEngine,
		api:         api,
		peers:       make(map[string]*webrtc.PeerConnection),
		videoTrack:  videoTrack,
		audioTrack:  audioTrack,
	}
}

// GetVideoTrack returns the local video track for writing RTP packets.
func (s *SFU) GetVideoTrack() *webrtc.TrackLocalStaticRTP {
	return s.videoTrack
}

// GetAudioTrack returns the local audio track for writing RTP packets.
func (s *SFU) GetAudioTrack() *webrtc.TrackLocalStaticRTP {
	return s.audioTrack
}

// OnPeerJoin sets a callback when a new peer connects.
func (s *SFU) OnPeerJoin(fn func(id string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPeerJoin = fn
}

// OnPeerLeave sets a callback when a peer disconnects.
func (s *SFU) OnPeerLeave(fn func(id string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPeerLeave = fn
}

// PeerCount returns the number of connected peers.
func (s *SFU) PeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

// HandleOffer processes a WebRTC SDP offer and returns an answer.
func (s *SFU) HandleOffer(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	peer, err := s.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{}, // No STUN/TURN needed on LAN
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create peer: %w", err)
	}

	peerID := fmt.Sprintf("peer-%d", time.Now().UnixNano())

	// Add video and audio tracks
	if _, err := peer.AddTrack(s.videoTrack); err != nil {
		peer.Close()
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}
	if _, err := peer.AddTrack(s.audioTrack); err != nil {
		peer.Close()
		return nil, fmt.Errorf("failed to add audio track: %w", err)
	}

	// Handle ICE candidates
	peer.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			log.Printf("ICE candidate for %s: %s", peerID, c.String())
		}
	})

	// Track connection state
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateConnected:
			log.Printf("Peer %s connected", peerID)
			s.mu.Lock()
			s.peers[peerID] = peer
			s.mu.Unlock()
			if s.onPeerJoin != nil {
				s.onPeerJoin(peerID)
			}
		case webrtc.PeerConnectionStateDisconnected, webrtc.PeerConnectionStateFailed:
			log.Printf("Peer %s disconnected", peerID)
			s.mu.Lock()
			delete(s.peers, peerID)
			s.mu.Unlock()
			peer.Close()
			if s.onPeerLeave != nil {
				s.onPeerLeave(peerID)
			}
		}
	})

	// Set remote description (the offer)
	if err := peer.SetRemoteDescription(offer); err != nil {
		peer.Close()
		return nil, fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	answer, err := peer.CreateAnswer(nil)
	if err != nil {
		peer.Close()
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	// Set local description
	if err := peer.SetLocalDescription(answer); err != nil {
		peer.Close()
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	return &answer, nil
}

// Close shuts down all peer connections.
func (s *SFU) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, peer := range s.peers {
		peer.Close()
		delete(s.peers, id)
	}
}
