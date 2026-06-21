# BIFROST — API Reference

Base URL: `http://<host>:8080`

## Endpoints

### GET `/`

Serves the web viewer HTML page.

**Response:**
- `200 OK`
- `Content-Type: text/html`
- Body: embedded `viewer.html` (see `web/templates/viewer.html`)

**Query Parameters:**
- `teacher=true` — Activates teacher dashboard mode (also auto-detected when accessing from `localhost`/`127.0.0.1`)

**Middleware:** Panic recovery

---

### GET `/stream`

MJPEG multipart video stream. Long-lived connection that pushes JPEG frames as they arrive from the capture pipeline.

**Response:**
- `200 OK`
- `Content-Type: multipart/x-mixed-replace; boundary=boundary`
- `Cache-Control: no-cache, private`
- `Pragma: no-cache`
- Body: Stream of JPEG frames separated by `--boundary`

**Frame Format:**
```
--boundary\r\n
Content-Type: image/jpeg\r\n
Content-Length: <N>\r\n
\r\n
<raw JPEG bytes>
\r\n
```

**Behavior:**
- Subscribes to the video Broadcaster (buffer size: 100)
- Unsubscribes when client disconnects (`r.Context().Done()`)
- Bandwidth tracked per-client via `Tracker.AddBytes()`
- Full buffer → frame dropped for that client (non-blocking publish)

**Middleware:** Panic recovery

---

### GET `/frame`

Returns the most recent JPEG frame as a single image. Used by the browser's refresh loop as an alternative to the multipart stream.

**Response:**
- `200 OK` — `Content-Type: image/jpeg`, `Cache-Control: no-cache, no-store, must-revalidate`
- `204 No Content` — No frame captured yet

**Behavior:**
- Returns a copy of `Broadcaster.lastFrame`
- Bandwidth tracked per-client

**Middleware:** Panic recovery

---

### GET `/audio`

**Deprecated.** Returns `410 Gone`. Audio was previously muxed separately but is no longer served.

---

### GET `/ping`

Client telemetry endpoint. The browser calls this every ~2 seconds to report device information and measure latency.

**Query Parameters:**

| Parameter    | Type   | Description                              |
| ------------ | ------ | ---------------------------------------- |
| `latency`    | int    | Round-trip latency in milliseconds       |
| `os`         | string | Client OS (e.g., `Linux`, `Android`)     |
| `browser`    | string | Browser name (e.g., `Chrome`, `Firefox`) |
| `resolution` | string | Screen resolution (e.g., `1920x1080`)    |
| `device`     | string | Device type (`desktop`, `mobile`)        |
| `gpu`        | string | GPU name (from WebGL renderer)           |
| `battery`    | int    | Battery percentage (0-100)               |
| `charging`   | string | `"true"` or `"false"`                    |

**Response:**
- `200 OK` — Empty body

**Behavior:**
- `Tracker.GetClient(ip)` creates or retrieves the client record
- Each parameter updates the corresponding field on `ClientInfo`
- Async background goroutines resolve hostname (DNS) and MAC address (`/proc/net/arp`)

**Middleware:** Panic recovery

---

### GET `/rejected`

Client-side rejection logging endpoint. Called by the browser when it detects it's running on Windows.

**Query Parameters:**

| Parameter | Type   | Description              |
| --------- | ------ | ------------------------ |
| `os`      | string | Operating system name    |
| `reason`  | string | Rejection reason         |

**Headers Used:**
- `User-Agent` — Logged for diagnostics

**Response:**
- `200 OK` — Empty body

**Behavior:**
- `Tracker.LogRejection()` adds to the rejection log (max 5 entries, newest first)

**Middleware:** Panic recovery

---

### POST `/push`

Browser-native screen capture bridge endpoint. Receives JPEG frames from the teacher's browser when using the Wayland bridge mode.

**Request:**
- Method: `POST`
- `Content-Type`: any (raw JPEG binary)
- Body: JPEG frame bytes

**Response:**
- `200 OK` — Frame published to all subscribers
- `405 Method Not Allowed` — Non-POST request
- `400 Bad Request` — Empty or unreadable body

**Behavior:**
- Sets `Broadcaster.header` to `"BRIDGE"` — signals the ffmpeg capture goroutine to stop (prevents dual-publishing)
- Publishes the received frame to all `/stream` subscribers via `Broadcaster.Publish()`

**Middleware:** Panic recovery

---

### GET `/stats`

JSON statistics endpoint. Used by the teacher dashboard UI to poll server state.

**Response:**
- `200 OK`
- `Content-Type: application/json`
- `Cache-Control: no-cache`

**Response Body:**
```json
{
  "total_transmitted": 1048576,
  "pub_total": 2097152,
  "pub_rate": 524288,
  "clients": [
    {
      "IP": "192.168.1.50",
      "Hostname": "student-pc.local",
      "MAC": "AA:BB:CC:DD:EE:FF",
      "Bytes": 524288,
      "Latency": 42,
      "OS": "Linux",
      "Browser": "Chrome",
      "Resolution": "1920x1080",
      "DevType": "desktop",
      "GPU": "Mesa Intel UHD Graphics",
      "BatPct": 85,
      "Charging": true,
      "Active": true
    }
  ],
  "rejections": [
    {
      "IP": "192.168.1.99",
      "OS": "Windows OS",
      "Reason": "server_guard",
      "Time": "2025-01-15T10:30:00Z",
      "UserAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
    }
  ],
  "uptime": "2h15m30s"
}
```

**Fields:**

| Field                | Type     | Description                              |
| -------------------- | -------- | ---------------------------------------- |
| `total_transmitted`  | int64    | Total bytes sent to all clients          |
| `pub_total`          | int64    | Total bytes published by Broadcaster     |
| `pub_rate`           | int64    | Bytes published in the last measurement cycle |
| `clients`            | array    | Active clients only                      |
| `rejections`         | array    | Recent rejections (max 5, newest first)  |
| `uptime`             | string   | Server uptime since start                |

**Middleware:** Panic recovery

---

### GET `/health`

Lightweight health check endpoint for monitoring.

**Response:**
- `200 OK`
- `Content-Type: application/json`

**Response Body:**
```json
{
  "streaming": true,
  "clients": 3
}
```

**Fields:**

| Field        | Type    | Description              |
| ------------ | ------- | ------------------------ |
| `streaming`  | bool    | Always `true` if running |
| `clients`    | int     | Number of active clients |

**Middleware:** Panic recovery

---

## Error Handling

- **Panic Recovery:** All routes (except `/audio`) are wrapped in `recoverMiddleware` which catches panics and returns `500 Internal Server Error`
- **404:** Unknown paths return the default `http.NotFound` response
- **TCP_NODELAY:** Enabled on all new connections via `ConnState` callback for reduced latency

## Client Middleware Pipeline

```
Request → recoverMiddleware → RejectWindows (guard) → Handler
```

- `/stream`, `/frame`: Guard + bandwidth tracking
- `/ping`, `/rejected`, `/push`, `/stats`, `/health`: Guard only (no bandwidth tracking)
- `/`: Guard only (serves HTML)
