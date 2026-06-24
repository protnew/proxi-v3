import { writable } from 'svelte/store';
import { apiFetch, createWebSocket, getIdentity } from './api';

export const webrtcState = writable('disconnected');
export const chatMessages = writable<{sender: string, text: string}[]>([]);
export const remoteStream = writable<MediaStream | null>(null);
export const localStream = writable<MediaStream | null>(null);

let peerConnection: RTCPeerConnection;
let dataChannel: RTCDataChannel;
let signalingWs: WebSocket;
export let pk: string;

export async function initNostr() {
    // We keep the function name initNostr so we don't break WebRtcMvp.svelte
    const idData = await getIdentity();
    pk = idData.npub;
    
    // Connect to local Go WebSocket
    signalingWs = createWebSocket('/ws');
    signalingWs.onmessage = async (event) => {
        try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'chat' && msg.text.startsWith('SIGNALING:')) {
                const sigData = JSON.parse(msg.text.substring(10));
                handleSignaling(sigData, msg.from);
            }
        } catch (e) {
            console.error("Failed to parse signaling message", e);
        }
    };
    return pk;
}

async function sendSignaling(targetPk: string, data: any) {
    const text = 'SIGNALING:' + JSON.stringify(data);
    await apiFetch('/api/messages', {
        method: 'POST',
        body: JSON.stringify({ to: targetPk, text })
    });
}

function setupPeerConnection(targetPk: string) {
    peerConnection = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
    });

    peerConnection.onicecandidate = (event) => {
        if (event.candidate) {
            sendSignaling(targetPk, { type: 'candidate', candidate: event.candidate });
        }
    };

    peerConnection.onconnectionstatechange = () => {
        webrtcState.set(peerConnection.connectionState);
    };

    peerConnection.ontrack = (event) => {
        remoteStream.set(event.streams[0]);
    };

    // Listen for data channel creation by the remote peer
    peerConnection.ondatachannel = (event) => {
        setupDataChannel(event.channel);
    };
}

function setupDataChannel(channel: RTCDataChannel) {
    dataChannel = channel;
    dataChannel.onopen = () => console.log('Data channel opened');
    dataChannel.onmessage = async (event) => {
        const msg = JSON.parse(event.data);
        if (msg.type === 'chat') {
            chatMessages.update(msgs => [...msgs, { sender: 'peer', text: msg.text }]);
        } else if (msg.type === 'vpn_request') {
            handleVpnRequest(msg);
        } else if (msg.type === 'vpn_response') {
            // Send back to Service Worker
            navigator.serviceWorker.controller?.postMessage(msg);
        }
    };
}

export async function connectToPeer(targetPk: string) {
    webrtcState.set('connecting');
    setupPeerConnection(targetPk);
    
    // Create data channel (we are the initiator)
    const channel = peerConnection.createDataChannel('proxi_channel');
    setupDataChannel(channel);

    // Get local stream for calls if available
    const unsubscribe = localStream.subscribe(stream => {
        if (stream) {
            stream.getTracks().forEach(track => peerConnection.addTrack(track, stream));
        }
    });
    // Let's just unsubscribe immediately, we add tracks once during connection
    unsubscribe();

    const offer = await peerConnection.createOffer();
    await peerConnection.setLocalDescription(offer);
    
    sendSignaling(targetPk, { type: 'offer', sdp: offer });
}

async function handleSignaling(msg: any, targetPk: string) {
    if (msg.type === 'offer') {
        setupPeerConnection(targetPk);
        await peerConnection.setRemoteDescription(new RTCSessionDescription(msg.sdp));
        
        const unsubscribe = localStream.subscribe(stream => {
            if (stream) {
                stream.getTracks().forEach(track => peerConnection.addTrack(track, stream));
            }
        });
        unsubscribe();

        const answer = await peerConnection.createAnswer();
        await peerConnection.setLocalDescription(answer);
        sendSignaling(targetPk, { type: 'answer', sdp: answer });
        
    } else if (msg.type === 'answer') {
        await peerConnection.setRemoteDescription(new RTCSessionDescription(msg.sdp));
    } else if (msg.type === 'candidate') {
        await peerConnection.addIceCandidate(new RTCIceCandidate(msg.candidate));
    }
}

export function sendChatMessage(text: string) {
    if (dataChannel && dataChannel.readyState === 'open') {
        dataChannel.send(JSON.stringify({ type: 'chat', text }));
        chatMessages.update(msgs => [...msgs, { sender: 'me', text }]);
    }
}

export async function startLocalVideo() {
    try {
        const stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: true });
        localStream.set(stream);
    } catch (e) {
        console.error("Camera access failed", e);
    }
}

export function stopLocalVideo() {
    localStream.update(stream => {
        if (stream) {
            stream.getTracks().forEach(track => track.stop());
        }
        return null;
    });
}

// VPN LOGIC
export function sendVpnRequest(request: any) {
    if (dataChannel && dataChannel.readyState === 'open') {
        dataChannel.send(JSON.stringify({ type: 'vpn_request', ...request }));
    }
}

async function handleVpnRequest(req: any) {
    try {
        const response = await fetch(req.url, {
            method: req.method,
            headers: req.headers,
            body: req.body
        });
        
        const blob = await response.blob();
        const reader = new FileReader();
        reader.readAsDataURL(blob);
        reader.onloadend = () => {
            const base64data = reader.result;
            if (dataChannel && dataChannel.readyState === 'open') {
                dataChannel.send(JSON.stringify({
                    type: 'vpn_response',
                    requestId: req.requestId,
                    status: response.status,
                    statusText: response.statusText,
                    headers: Array.from(response.headers.entries()),
                    body: base64data
                }));
            }
        };
    } catch (e) {
        if (dataChannel && dataChannel.readyState === 'open') {
            dataChannel.send(JSON.stringify({
                type: 'vpn_response',
                requestId: req.requestId,
                status: 500,
                error: e.toString()
            }));
        }
    }
}
