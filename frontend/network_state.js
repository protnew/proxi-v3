export const NetworkState = {
    CONNECTED: 'CONNECTED',
    DISCONNECTED: 'DISCONNECTED',
    RECONNECTING: 'RECONNECTING',
    OFFLINE_QUEUE: 'OFFLINE_QUEUE'
};

export class NostrConnectionManager {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.state = NetworkState.DISCONNECTED;
        this.reconnectAttempts = 0;
        this.maxReconnectDelay = 30000; // 30s
        this.offlineQueue = [];
        
        // Слушаем системные события сети (WiFi -> LTE -> Offline)
        // Это жизненно важно для мгновенного реконнекта при смене вышек связи
        window.addEventListener('online', () => this.handleNetworkChange(true));
        window.addEventListener('offline', () => this.handleNetworkChange(false));
    }

    connect() {
        if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
            return;
        }

        this.state = NetworkState.RECONNECTING;
        console.log(`[Nostr] Connecting to ${this.url}...`);
        
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
            console.log('[Nostr] Connected');
            this.state = NetworkState.CONNECTED;
            this.reconnectAttempts = 0;
            this.flushOfflineQueue();
        };

        this.ws.onclose = (e) => {
            console.log('[Nostr] Disconnected', e.reason);
            this.state = NetworkState.DISCONNECTED;
            this.scheduleReconnect();
        };

        this.ws.onerror = (err) => {
            console.error('[Nostr] WebSocket Error', err);
            // onclose will automatically be called by the browser
        };
    }

    handleNetworkChange(isOnline) {
        if (isOnline) {
            console.log('[Network] Browser went ONLINE (WiFi/LTE restored). Reconnecting immediately.');
            this.reconnectAttempts = 0;
            this.connect();
        } else {
            console.log('[Network] Browser went OFFLINE. Queuing messages.');
            this.state = NetworkState.OFFLINE_QUEUE;
            if (this.ws) {
                this.ws.close();
            }
        }
    }

    scheduleReconnect() {
        if (!navigator.onLine) {
            return; // Ждем системного события 'online'
        }

        // Exponential backoff алгоритм
        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), this.maxReconnectDelay);
        this.reconnectAttempts++;
        
        console.log(`[Nostr] Scheduling reconnect in ${delay}ms... (Attempt ${this.reconnectAttempts})`);
        setTimeout(() => this.connect(), delay);
    }

    send(message) {
        if (this.state === NetworkState.CONNECTED && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            console.log('[Nostr] Queued message due to disconnect (Network Chaos mitigation)');
            this.offlineQueue.push(message);
        }
    }

    flushOfflineQueue() {
        if (this.offlineQueue.length === 0) return;
        
        console.log(`[Nostr] Flushing ${this.offlineQueue.length} queued messages after reconnect...`);
        const messages = [...this.offlineQueue];
        this.offlineQueue = [];
        
        messages.forEach(msg => this.send(msg));
    }
}
