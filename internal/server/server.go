package server

import (
	"bifrost/internal/stream"
	"bifrost/internal/tracker"
	webrtc "bifrost/internal/webrtc"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var startTime = time.Now()

func getIP(r *http.Request) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func New(
	tr *tracker.Tracker,
	broadcaster *stream.Broadcaster,
	signalingServer *webrtc.SignalingServer,
	playerHTML string,
) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>BIFROST</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0a0000;color:#fff;font-family:'Courier New',monospace;height:100vh;overflow:hidden}
#stream{position:fixed;top:0;left:0;width:100vw;height:100vh;object-fit:contain;z-index:0}
.hud{position:fixed;top:10px;left:10px;z-index:10;background:rgba(170,0,0,0.85);
  border:1px solid #ff3333;padding:6px 12px;border-radius:4px;font-size:13px;font-weight:bold}
.hud .dot{display:inline-block;width:8px;height:8px;background:#0f0;border-radius:50%;
  margin-right:6px;animation:blink 1s infinite alternate}
@keyframes blink{from{opacity:.3}to{opacity:1}}
#bridge-btn{position:fixed;top:10px;right:10px;z-index:10;background:#aa0000;color:#fff;
  border:1px solid #ff3333;padding:10px 20px;cursor:pointer;font-family:inherit;
  font-weight:bold;font-size:14px;border-radius:4px}
#bridge-btn:hover{background:#ff3333}
#status{position:fixed;bottom:10px;left:10px;z-index:10;font-size:11px;color:#666}
</style>
</head><body>
<div class="hud"><span class="dot"></span><span id="label">STUDENT VIEW</span></div>
<img id="stream">
<button id="bridge-btn" onclick="startBridge()">START BROADCAST</button>
<div id="status"></div>
<script>
var img=document.getElementById("stream");
var btn=document.getElementById("bridge-btn");
var lbl=document.getElementById("label");
var stat=document.getElementById("status");
var bridge=false;

function poll(){
  if(!bridge){
    var x=new Image();
    x.onload=function(){img.src=x.src;setTimeout(poll,66)};
    x.onerror=function(){setTimeout(poll,1000)};
    x.src="/frame?t="+Date.now();
  }
}
poll();

function startBridge(){
  if(!navigator.mediaDevices||!navigator.mediaDevices.getDisplayMedia){
    alert("Screen capture requires HTTPS or localhost.");return;
  }
  navigator.mediaDevices.getDisplayMedia({video:{frameRate:15,cursor:"always"},audio:false})
  .then(function(stream){
    bridge=true;
    btn.style.display="none";
    lbl.innerText="BROADCASTING";
    document.querySelector(".dot").style.background="#ff0000";
    var video=document.createElement("video");
    video.srcObject=stream;video.play();
    var canvas=document.createElement("canvas");
    var ctx=canvas.getContext("2d");
    function push(){
      if(!bridge)return;
      canvas.width=video.videoWidth;canvas.height=video.videoHeight;
      ctx.drawImage(video,0,0);
      canvas.toBlob(function(blob){
        fetch("/push",{method:"POST",body:blob}).then(function(r){
          stat.innerText="Frame: "+Math.round(blob.size/1024)+"KB";
        }).catch(function(){});
        setTimeout(push,66);
      },"image/jpeg",0.8);
    }
    push();
    stream.getTracks()[0].onended=function(){bridge=false;location.reload()};
  }).catch(function(e){alert("Capture failed: "+e.message)});
}
</script>
</body></html>`)
	}))

	mux.HandleFunc("/frame", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		tr.GetClient(ip)
		log.Printf("[/frame] request from %s", r.RemoteAddr)

		frame := broadcaster.GetLastFrame()
		if frame == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", strconv.Itoa(len(frame)))
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		n, err := w.Write(frame)
		if err == nil && n > 0 {
			tr.AddBytes(ip, int64(n))
		}
	}))

	mux.HandleFunc("/stream", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		tr.GetClient(ip)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=boundary")
		w.Header().Set("Cache-Control", "no-cache, private")
		w.Header().Set("Pragma", "no-cache")

		ch := broadcaster.Subscribe(100)
		defer broadcaster.Unsubscribe(ch)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case chunk, ok := <-ch:
				if !ok {
					return
				}
				header := fmt.Sprintf("--boundary\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(chunk))
				n1, _ := fmt.Fprint(w, header)
				n2, _ := w.Write(chunk)
				n3, _ := fmt.Fprint(w, "\r\n")

				tr.AddBytes(ip, int64(n1+n2+n3))
				flusher.Flush()
			}
		}
	}))

	mux.HandleFunc("/ping", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		client := tr.GetClient(ip)

		q := r.URL.Query()
		if lat, err := strconv.Atoi(q.Get("latency")); err == nil {
			client.Latency = lat
		}
		if os := q.Get("os"); os != "" {
			client.OS = os
		}
		if browser := q.Get("browser"); browser != "" {
			client.Browser = browser
		}
		if res := q.Get("resolution"); res != "" {
			client.Resolution = res
		}
		if dev := q.Get("device"); dev != "" {
			client.DevType = dev
		}
		if gpu := q.Get("gpu"); gpu != "" {
			client.GPU = gpu
		}
		if bat, err := strconv.Atoi(q.Get("battery")); err == nil {
			client.BatPct = bat
		}
		if charging := q.Get("charging"); charging != "" {
			client.Charging = (charging == "true")
		}

		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/rejected", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		os := r.URL.Query().Get("os")
		reason := r.URL.Query().Get("reason")
		ua := r.Header.Get("User-Agent")
		tr.LogRejection(ip, os, reason, ua)
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/push", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		broadcaster.SetHeader([]byte("BRIDGE"))
		broadcaster.Publish(body)
		w.WriteHeader(http.StatusOK)
	}))

	mux.HandleFunc("/stats", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		tr.RLock()
		defer tr.RUnlock()

		activeClients := make([]tracker.ClientInfo, 0)
		for _, c := range tr.Clients {
			if c.Active {
				activeClients = append(activeClients, *c)
			}
		}

		res := map[string]interface{}{
			"total_transmitted": tr.TotalBytes,
			"pub_total":         broadcaster.Total,
			"pub_rate":          broadcaster.GetPubRate(),
			"clients":           activeClients,
			"rejections":        tr.Rejections,
			"uptime":            time.Since(startTime).String(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		json.NewEncoder(w).Encode(res)
	}))

	mux.HandleFunc("/health", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
		tr.RLock()
		activeClients := 0
		for _, c := range tr.Clients {
			if c.Active {
				activeClients++
			}
		}
		tr.RUnlock()

		res := map[string]interface{}{
			"streaming": true,
			"clients":   activeClients,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	}))

	// WebRTC signaling endpoint
	if signalingServer != nil {
		signalingServer.RegisterRoutes(mux)
	}

	// Player page for students
	if playerHTML != "" {
		mux.HandleFunc("/watch", recoverMiddleware(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			fmt.Fprint(w, playerHTML)
		}))
	}

	return &http.Server{
		Handler: mux,
		ConnState: func(conn net.Conn, state http.ConnState) {
			if state == http.StateNew {
				if tc, ok := conn.(*net.TCPConn); ok {
					tc.SetNoDelay(true)
				}
			}
		},
	}
}

func recoverMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered in handler: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}
