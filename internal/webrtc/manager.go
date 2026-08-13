package webrtc

import (
	"log"
	"sync"

	"github.com/pion/webrtc/v4"
)

type Manager struct {
	mu          sync.RWMutex
	videoTrack  *webrtc.TrackLocalStaticRTP
	audioTrack  *webrtc.TrackLocalStaticRTP
	peers       map[string]*webrtc.PeerConnection
	icePending  map[string][]webrtc.ICECandidateInit
}

func NewManager() *Manager {
	return &Manager{
		peers:      make(map[string]*webrtc.PeerConnection),
		icePending: make(map[string][]webrtc.ICECandidateInit),
	}
}

func (m *Manager) SetVideoTrack(track *webrtc.TrackLocalStaticRTP) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.videoTrack = track
}

func (m *Manager) SetAudioTrack(track *webrtc.TrackLocalStaticRTP) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audioTrack = track
}

func (m *Manager) CreatePeerFromOffer(peerID, sdp string) (string, error) {
	// LAN-only: no STUN/TURN — host candidates are sufficient
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
		// Set network type to ensure LAN interface candidates are generated
		SDPSemantics: webrtc.SDPSemanticsUnifiedPlan,
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return "", err
	}

	m.mu.Lock()
	m.peers[peerID] = peerConnection
	m.icePending[peerID] = make([]webrtc.ICECandidateInit, 0)
	m.mu.Unlock()

	// Add local tracks
	if m.videoTrack != nil {
		rtpSender, err := peerConnection.AddTrack(m.videoTrack)
		if err != nil {
			peerConnection.Close()
			return "", err
		}
		go readRTCP(rtpSender)
	}

	if m.audioTrack != nil {
		rtpSender, err := peerConnection.AddTrack(m.audioTrack)
		if err != nil {
			peerConnection.Close()
			return "", err
		}
		go readRTCP(rtpSender)
	}

	// Handle ICE candidates from pion
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		init := c.ToJSON()
		m.mu.Lock()
		m.icePending[peerID] = append(m.icePending[peerID], init)
		m.mu.Unlock()
	})

	// Handle connection state
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[WebRTC] Peer %s state: %s", peerID, state.String())
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			m.removePeer(peerID)
		}
	})

	// Set remote description (offer)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		m.removePeer(peerID)
		return "", err
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		m.removePeer(peerID)
		return "", err
	}

	// Set local description
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		m.removePeer(peerID)
		return "", err
	}

	log.Printf("[WebRTC] Created peer %s", peerID)
	return answer.SDP, nil
}

func (m *Manager) AddICECandidate(peerID string, candidate webrtc.ICECandidateInit) error {
	m.mu.RLock()
	peer, ok := m.peers[peerID]
	m.mu.RUnlock()

	if !ok {
		// Queue it for later
		m.mu.Lock()
		m.icePending[peerID] = append(m.icePending[peerID], candidate)
		m.mu.Unlock()
		return nil
	}

	return peer.AddICECandidate(candidate)
}

func (m *Manager) PendingICE(peerID string) []webrtc.ICECandidateInit {
	m.mu.Lock()
	defer m.mu.Unlock()

	candidates := m.icePending[peerID]
	m.icePending[peerID] = make([]webrtc.ICECandidateInit, 0)
	return candidates
}

func (m *Manager) PeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

func (m *Manager) removePeer(peerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if peer, ok := m.peers[peerID]; ok {
		peer.Close()
		delete(m.peers, peerID)
	}
	delete(m.icePending, peerID)
}

func readRTCP(sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		_, _, err := sender.Read(buf)
		if err != nil {
			return
		}
	}
}
