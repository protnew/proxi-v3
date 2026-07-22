/**
 * Stream.js — Live video streaming via WebSocket binary frames
 * Sprint 3 — Task 1
 *
 * Streamer captures video from webcam, encodes frames, and sends them
 * as binary WS messages (0x03 prefix). Viewers receive and display frames.
 */

class LiveStream {
  constructor(wsUrl, userId) {
    this.wsUrl = wsUrl;
    this.userId = userId || 'anonymous';
    this.ws = null;
    this.streamId = null;
    this.isStreaming = false;
    this.isViewing = false;
    this.mediaStream = null;
    this.videoElement = null;
    this.canvasElement = null;
    this.frameNumber = 0;
    this.frameInterval = null;
    this.frameRate = 15; // fps
    this.quality = 0.7; // JPEG quality
    this.onFrame = null; // callback for received frames
    this.onStreamEnd = null;
    this.onViewerCount = null;
    this.apiUrl = ''; // will be derived from wsUrl
  }

  // Derive API URL from WS URL
  _getApiUrl() {
    if (this.apiUrl) return this.apiUrl;
    const url = new URL(this.wsUrl);
    this.apiUrl = `${url.protocol === 'wss:' ? 'https' : 'http'}://${url.host}`;
    return this.apiUrl;
  }

  // Connect WebSocket
  async connect() {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.wsUrl);
      ws.binaryType = 'arraybuffer';
      ws.onopen = () => {
        this.ws = ws;
        console.log('[stream] WS connected');
        resolve();
      };
      ws.onerror = (e) => reject(e);
      ws.onmessage = (e) => this._handleMessage(e);
      ws.onclose = () => {
        console.log('[stream] WS closed');
        this.isStreaming = false;
        this.isViewing = false;
      };
    });
  }

  // Handle incoming WS messages
  _handleMessage(event) {
    if (event.data instanceof ArrayBuffer) {
      const data = new Uint8Array(event.data);
      // Check if it's a stream frame (0x03 prefix)
      if (data[0] === 0x03 && data.length > 5) {
        this._handleStreamFrame(data);
      }
    } else if (typeof event.data === 'string') {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'stream-frame') {
          // Metadata about stream frame
        }
      } catch (e) {
        // Ignore non-JSON text
      }
    }
  }

  // Handle a received stream frame
  _handleStreamFrame(data) {
    // Parse stream frame: [0x03] [4-byte ID len] [stream ID] [4-byte frame#] [payload]
    const idLen = (data[1] << 24) | (data[2] << 16) | (data[3] << 8) | data[4];
    if (data.length < 5 + idLen + 4) return;

    const streamId = new TextDecoder().decode(data.slice(5, 5 + idLen));
    const frameNumber = (data[5 + idLen] << 24) | (data[5 + idLen + 1] << 16) |
                        (data[5 + idLen + 2] << 8) | data[5 + idLen + 3];
    const payload = data.slice(5 + idLen + 4);

    // Create blob from JPEG payload
    const blob = new Blob([payload], { type: 'image/jpeg' });
    const url = URL.createObjectURL(blob);

    if (this.onFrame) {
      this.onFrame({
        streamId,
        frameNumber,
        imageUrl: url,
        timestamp: Date.now()
      });
    }

    // Display on video element if set
    if (this.videoElement && this.isViewing) {
      const img = new Image();
      img.onload = () => {
        const ctx = this.videoElement.getContext('2d');
        if (ctx) {
          ctx.drawImage(img, 0, 0, this.videoElement.width, this.videoElement.height);
        }
        URL.revokeObjectURL(url);
      };
      img.src = url;
    }
  }

  // ========== Streamer API ==========

  // Create a stream via REST API
  async createStream(channelName) {
    const resp = await fetch(`${this._getApiUrl()}/api/stream/create`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        channelName: channelName || 'Live Stream',
        streamerId: this.userId
      })
    });
    const data = await resp.json();
    if (data.id) {
      this.streamId = data.id;
    }
    return data;
  }

  // Start streaming from webcam
  async goLive(canvasEl, channelName) {
    if (this.isStreaming) {
      console.warn('[stream] Already streaming');
      return;
    }

    // Create stream
    const streamInfo = await this.createStream(channelName);
    if (!streamInfo.id) {
      throw new Error('Failed to create stream');
    }
    this.streamId = streamInfo.id;

    // Get camera
    try {
      this.mediaStream = await navigator.mediaDevices.getUserMedia({
        video: { width: 640, height: 480, frameRate: { ideal: this.frameRate } },
        audio: false
      });
    } catch (e) {
      throw new Error('Camera access denied: ' + e.message);
    }

    // Setup canvas for frame capture
    this.canvasElement = canvasEl || document.createElement('canvas');
    this.canvasElement.width = 640;
    this.canvasElement.height = 480;
    const ctx = this.canvasElement.getContext('2d');

    // Create hidden video element to play camera stream
    const video = document.createElement('video');
    video.srcObject = this.mediaStream;
    video.play();

    this.isStreaming = true;
    this.frameNumber = 0;

    // Send frames at configured frame rate
    this.frameInterval = setInterval(() => {
      if (!this.isStreaming) return;

      // Draw current video frame to canvas
      ctx.drawImage(video, 0, 0, this.canvasElement.width, this.canvasElement.height);

      // Convert to JPEG
      this.canvasElement.toBlob((blob) => {
        if (!blob || !this.ws || this.ws.readyState !== WebSocket.OPEN) return;

        blob.arrayBuffer().then((buffer) => {
          const payload = new Uint8Array(buffer);
          const frame = this._encodeStreamFrame(this.streamId, this.frameNumber++, payload);
          this.ws.send(frame);
        });
      }, 'image/jpeg', this.quality);
    }, 1000 / this.frameRate);

    console.log(`[stream] Live! Stream ID: ${this.streamId}`);
    return streamInfo;
  }

  // Encode a binary stream frame
  _encodeStreamFrame(streamId, frameNumber, payload) {
    const idBytes = new TextEncoder().encode(streamId);
    const idLen = idBytes.length;

    // [0x03] [4-byte ID len] [stream ID] [4-byte frame#] [payload]
    const frame = new Uint8Array(1 + 4 + idLen + 4 + payload.length);
    frame[0] = 0x03;
    frame[1] = (idLen >> 24) & 0xFF;
    frame[2] = (idLen >> 16) & 0xFF;
    frame[3] = (idLen >> 8) & 0xFF;
    frame[4] = idLen & 0xFF;
    frame.set(idBytes, 5);
    const offset = 5 + idLen;
    frame[offset] = (frameNumber >> 24) & 0xFF;
    frame[offset + 1] = (frameNumber >> 16) & 0xFF;
    frame[offset + 2] = (frameNumber >> 8) & 0xFF;
    frame[offset + 3] = frameNumber & 0xFF;
    frame.set(payload, offset + 4);

    return frame.buffer;
  }

  // Stop streaming
  async stopStream() {
    this.isStreaming = false;
    if (this.frameInterval) {
      clearInterval(this.frameInterval);
      this.frameInterval = null;
    }
    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach(t => t.stop());
      this.mediaStream = null;
    }
    if (this.streamId) {
      try {
        await fetch(`${this._getApiUrl()}/api/stream/end`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ streamId: this.streamId })
        });
      } catch (e) {
        console.warn('[stream] End stream API error:', e);
      }
      this.streamId = null;
    }
    console.log('[stream] Stopped');
  }

  // ========== Viewer API ==========

  // List active streams
  async listStreams() {
    const resp = await fetch(`${this._getApiUrl()}/api/stream/list`);
    return resp.json();
  }

  // Subscribe to a stream
  async subscribe(streamId) {
    const resp = await fetch(`${this._getApiUrl()}/api/stream/subscribe`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ streamId, userId: this.userId })
    });
    const data = await resp.json();
    this.isViewing = true;
    return data;
  }

  // Set video element for rendering
  setVideoElement(canvasEl) {
    this.videoElement = canvasEl;
  }

  // Stop viewing
  stopViewing() {
    this.isViewing = false;
    this.videoElement = null;
  }

  // ========== Cleanup ==========

  destroy() {
    this.stopStream();
    this.stopViewing();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

// ========== Stream UI Component ==========

class StreamUI {
  constructor(containerId) {
    this.container = document.getElementById(containerId);
    this.stream = null;
    this.init();
  }

  init() {
    if (!this.container) return;

    this.container.innerHTML = `
      <div class="stream-panel">
        <h3>📺 Live Streams</h3>
        <div class="stream-controls">
          <button id="goLiveBtn" class="btn-primary">🔴 Go Live</button>
          <button id="stopLiveBtn" class="btn-danger" style="display:none">⏹ Stop</button>
          <button id="refreshStreamsBtn" class="btn-secondary">🔄 Refresh</button>
        </div>
        <div id="streamList" class="stream-list"></div>
        <div id="streamView" style="display:none">
          <canvas id="streamCanvas" width="640" height="480"></canvas>
        </div>
      </div>
    `;

    document.getElementById('goLiveBtn').onclick = () => this.goLive();
    document.getElementById('stopLiveBtn').onclick = () => this.stopLive();
    document.getElementById('refreshStreamsBtn').onclick = () => this.refreshStreams();

    // Connect WS
    const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${wsProto}//${location.host}/ws?userId=stream-user`;
    this.stream = new LiveStream(wsUrl, 'stream-user');
    this.stream.connect().then(() => {
      this.refreshStreams();
    });

    // Set canvas for viewer
    this.stream.setVideoElement(document.getElementById('streamCanvas'));
    this.stream.onFrame = (frame) => {
      document.getElementById('streamView').style.display = 'block';
    };
  }

  async goLive() {
    try {
      const canvas = document.getElementById('streamCanvas');
      document.getElementById('streamView').style.display = 'block';
      await this.stream.goLive(canvas, 'My Stream');
      document.getElementById('goLiveBtn').style.display = 'none';
      document.getElementById('stopLiveBtn').style.display = 'inline';
    } catch (e) {
      alert('Failed to go live: ' + e.message);
    }
  }

  async stopLive() {
    await this.stream.stopStream();
    document.getElementById('goLiveBtn').style.display = 'inline';
    document.getElementById('stopLiveBtn').style.display = 'none';
    document.getElementById('streamView').style.display = 'none';
  }

  async refreshStreams() {
    const data = await this.stream.listStreams();
    const listEl = document.getElementById('streamList');
    if (!listEl) return;

    if (!data.streams || data.streams.length === 0) {
      listEl.innerHTML = '<p>No active streams</p>';
      return;
    }

    listEl.innerHTML = data.streams.map(s => `
      <div class="stream-item" data-id="${s.id}">
        <span>📺 ${s.channelName || 'Live'} — by ${s.streamerId}</span>
        <span>👁 ${s.viewerCount || 0}</span>
        <button onclick="streamUI.watchStream('${s.id}')">Watch</button>
      </div>
    `).join('');
  }

  async watchStream(streamId) {
    await this.stream.subscribe(streamId);
    document.getElementById('streamView').style.display = 'block';
  }
}

// Export for use
window.LiveStream = LiveStream;
window.StreamUI = StreamUI;
