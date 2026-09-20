package com.indestructible.messenger.messenger

import android.content.Context
import android.util.Log
import androidx.compose.runtime.mutableStateOf
import org.json.JSONObject
import org.webrtc.AudioSource
import org.webrtc.AudioTrack
import org.webrtc.DataChannel
import org.webrtc.DefaultVideoDecoderFactory
import org.webrtc.DefaultVideoEncoderFactory
import org.webrtc.EglBase
import org.webrtc.IceCandidate
import org.webrtc.MediaConstraints
import org.webrtc.MediaStream
import org.webrtc.PeerConnection
import org.webrtc.PeerConnectionFactory
import org.webrtc.RtpReceiver
import org.webrtc.SdpObserver
import org.webrtc.SessionDescription
import org.webrtc.SurfaceViewRenderer
import org.webrtc.VideoTrack

/**
 * 1:1 WebRTC calls. Signaling is wire-compatible with the PWA client:
 * signals ride `key_exchange` WS messages where `publicKey` carries a JSON
 * signal: call-offer/call-answer {sdp}, call-ice {candidate},
 * call-reject, call-end.
 */
object CallManager {
    enum class State { IDLE, OUTGOING, INCOMING, CONNECTING, CONNECTED, ENDED }

    val state = mutableStateOf(State.IDLE)
    val peerPub = mutableStateOf("")
    val muted = mutableStateOf(false)
    val videoEnabled = mutableStateOf(false)
    val error = mutableStateOf<String?>(null)

    // Wired by ChatViewModel: (peer, signalJson) -> send key_exchange
    var sendSignal: ((String, JSONObject) -> Unit)? = null

    private var factory: PeerConnectionFactory? = null
    private var pc: PeerConnection? = null
    private var eglBase: EglBase? = null
    private var audioSource: AudioSource? = null
    private var localAudio: AudioTrack? = null
    private var localVideo: VideoTrack? = null
    private var videoCapturer: org.webrtc.VideoCapturer? = null
    private var pendingOfferSdp: String? = null
    private var pendingVideo = false
    private var remoteAudioTrack: AudioTrack? = null
    var remoteVideoTrack: VideoTrack? = null
        private set
    var localVideoTrack: VideoTrack? = null
        private set
    private var initialized = false
    private var appCtx: Context? = null

    private val mainHandler = android.os.Handler(android.os.Looper.getMainLooper())
    private fun onMain(block: () -> Unit) = mainHandler.post(block)

    @Synchronized
    fun init(ctx: Context) {
        if (initialized) return
        appCtx = ctx.applicationContext
        PeerConnectionFactory.initialize(
            PeerConnectionFactory.InitializationOptions.builder(ctx.applicationContext)
                .setEnableInternalTracer(false)
                .createInitializationOptions()
        )
        eglBase = EglBase.create()
        factory = PeerConnectionFactory.builder()
            .setVideoEncoderFactory(DefaultVideoEncoderFactory(eglBase!!.eglBaseContext, true, true))
            .setVideoDecoderFactory(DefaultVideoDecoderFactory(eglBase!!.eglBaseContext))
            .createPeerConnectionFactory()
        initialized = true
    }

    private fun rtcConfig() = PeerConnection.RTCConfiguration(
        listOf(
            PeerConnection.IceServer.builder("stun:stun.l.google.com:19302").createIceServer(),
            PeerConnection.IceServer.builder("stun:stun1.l.google.com:19302").createIceServer(),
        )
    )

    private fun signal(type: String, extra: JSONObject.() -> Unit = {}) {
        val to = peerPub.value
        if (to.isEmpty()) return
        sendSignal?.invoke(to, JSONObject().apply {
            put("type", type)
            extra()
        })
    }

    private fun createPeerConnection(video: Boolean) {
        val f = factory ?: return
        pc = f.createPeerConnection(rtcConfig(), object : PeerConnection.Observer {
            override fun onIceCandidate(candidate: IceCandidate) {
                signal("call-ice") {
                    put("candidate", JSONObject().apply {
                        put("sdpMid", candidate.sdpMid)
                        put("sdpMLineIndex", candidate.sdpMLineIndex)
                        put("candidate", candidate.sdp)
                    })
                }
            }
            override fun onAddTrack(receiver: RtpReceiver, streams: Array<out MediaStream>) {
                val track = receiver.track()
                when (track) {
                    is AudioTrack -> { remoteAudioTrack = track; track.setEnabled(true) }
                    is VideoTrack -> { remoteVideoTrack = track }
                }
                onMain {
                    if (state.value == State.CONNECTING || state.value == State.OUTGOING) {
                        state.value = State.CONNECTED
                    }
                }
            }
            override fun onIceConnectionChange(newState: PeerConnection.IceConnectionState) {
                onMain {
                    when (newState) {
                        PeerConnection.IceConnectionState.CONNECTED,
                        PeerConnection.IceConnectionState.COMPLETED -> state.value = State.CONNECTED
                        PeerConnection.IceConnectionState.DISCONNECTED,
                        PeerConnection.IceConnectionState.FAILED -> endCallLocal(notify = false)
                        else -> {}
                    }
                }
            }
            override fun onSignalingChange(s: PeerConnection.SignalingState) {}
            override fun onIceConnectionReceivingChange(b: Boolean) {}
            override fun onIceGatheringChange(s: PeerConnection.IceGatheringState) {}
            override fun onRemoveStream(s: MediaStream) {}
            override fun onDataChannel(dc: DataChannel) {}
            override fun onRenegotiationNeeded() {}
            override fun onAddStream(s: MediaStream) {}
            override fun onIceCandidatesRemoved(c: Array<out IceCandidate>) {}
        })

        // Audio always.
        audioSource = f.createAudioSource(MediaConstraints())
        localAudio = f.createAudioTrack("audio0", audioSource)
        pc?.addTrack(localAudio, listOf("stream0"))

        if (video) {
            startVideoCapture()
        }
    }

    private fun startVideoCapture() {
        val ctx = appCtx ?: return
        val f = factory ?: return
        val capturer = createCameraCapturer() ?: return
        val surfaceHelper = org.webrtc.SurfaceTextureHelper.create("CaptureThread", eglBase!!.eglBaseContext)
        val videoSource = f.createVideoSource(capturer.isScreencast)
        capturer.initialize(surfaceHelper, ctx, videoSource.capturerObserver)
        capturer.startCapture(640, 480, 24)
        localVideo = f.createVideoTrack("video0", videoSource)
        pc?.addTrack(localVideo, listOf("stream0"))
        videoCapturer = capturer
        videoEnabled.value = true
    }

    private fun createCameraCapturer(): org.webrtc.VideoCapturer? {
        val enumerator = org.webrtc.Camera2Enumerator(appCtx ?: return null)
        for (name in enumerator.deviceNames) {
            if (enumerator.isFrontFacing(name)) {
                return enumerator.createCapturer(name, null)
            }
        }
        for (name in enumerator.deviceNames) {
            if (!enumerator.isFrontFacing(name)) {
                return enumerator.createCapturer(name, null)
            }
        }
        return null
    }

    /** Outgoing call. `video` = camera on. */
    fun startCall(ctx: Context, peer: String, video: Boolean) {
        if (state.value != State.IDLE) return
        init(ctx)
        peerPub.value = peer
        pendingVideo = video
        muted.value = false
        videoEnabled.value = video
        error.value = null
        state.value = State.OUTGOING
        Thread {
            createPeerConnection(video)
            val constraints = MediaConstraints().apply {
                mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveAudio", "true"))
                mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveVideo", if (video) "true" else "false"))
            }
            pc?.createOffer(object : SdpObserver {
                override fun onCreateSuccess(sdp: SessionDescription) {
                    pc?.setLocalDescription(object : SdpObserver {
                        override fun onSetSuccess() {
                            signal("call-offer") { put("sdp", sdp.description); put("video", video) }
                            onMain { state.value = State.CONNECTING }
                        }
                        override fun onSetFailure(e: String) { onMain { fail(e) } }
                        override fun onCreateSuccess(s: SessionDescription) {}
                        override fun onCreateFailure(e: String) {}
                    }, sdp)
                }
                override fun onCreateFailure(e: String) { onMain { fail(e) } }
                override fun onSetSuccess() {}
                override fun onSetFailure(e: String) {}
            }, constraints)
        }.start()
    }

    /** Incoming call accepted by user. */
    fun acceptCall(ctx: Context) {
        val sdp = pendingOfferSdp ?: return
        init(ctx)
        onMain { state.value = State.CONNECTING }
        Thread {
            createPeerConnection(pendingVideo)
            pc?.setRemoteDescription(object : SdpObserver {
                override fun onSetSuccess() {
                    val constraints = MediaConstraints().apply {
                        mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveAudio", "true"))
                        mandatory.add(MediaConstraints.KeyValuePair("OfferToReceiveVideo", if (pendingVideo) "true" else "false"))
                    }
                    pc?.createAnswer(object : SdpObserver {
                        override fun onCreateSuccess(answer: SessionDescription) {
                            pc?.setLocalDescription(object : SdpObserver {
                                override fun onSetSuccess() {
                                    signal("call-answer") { put("sdp", answer.description) }
                                }
                                override fun onSetFailure(e: String) { onMain { fail(e) } }
                                override fun onCreateSuccess(s: SessionDescription) {}
                                override fun onCreateFailure(e: String) {}
                            }, answer)
                        }
                        override fun onCreateFailure(e: String) { onMain { fail(e) } }
                        override fun onSetSuccess() {}
                        override fun onSetFailure(e: String) {}
                    }, constraints)
                }
                override fun onSetFailure(e: String) { onMain { fail(e) } }
                override fun onCreateSuccess(s: SessionDescription) {}
                override fun onCreateFailure(e: String) {}
            }, SessionDescription(SessionDescription.Type.OFFER, sdp))
            pendingOfferSdp = null
        }.start()
    }

    fun rejectCall() {
        signal("call-reject")
        endCallLocal(notify = false)
    }

    fun endCall() {
        signal("call-end")
        endCallLocal(notify = false)
    }

    fun toggleMute(): Boolean {
        val m = !muted.value
        localAudio?.setEnabled(!m)
        muted.value = m
        return m
    }

    fun toggleVideo(): Boolean {
        val on = !videoEnabled.value
        if (on && localVideo == null) {
            startVideoCapture()
        } else {
            localVideo?.setEnabled(on)
        }
        videoEnabled.value = on
        return on
    }

    private fun fail(e: String) {
        error.value = e
        endCallLocal(notify = false)
    }

    private fun endCallLocal(notify: Boolean) {
        if (notify) signal("call-end")
        Thread {
            try { videoCapturer?.stopCapture() } catch (_: Exception) {}
            videoCapturer?.dispose(); videoCapturer = null
            localAudio?.dispose(); localAudio = null
            localVideo?.dispose(); localVideo = null
            audioSource?.dispose(); audioSource = null
            remoteAudioTrack = null; remoteVideoTrack = null; localVideoTrack = null
            pc?.close(); pc = null
        }.start()
        pendingOfferSdp = null
        peerPub.value = ""
        muted.value = false
        videoEnabled.value = false
        if (state.value != State.IDLE) state.value = State.ENDED
        onMain { if (state.value == State.ENDED) state.value = State.IDLE }
    }

    /** Incoming key_exchange signal from the WS listener. */
    fun handleSignal(from: String, signalJson: String) {
        try {
            val sig = JSONObject(signalJson)
            when (sig.optString("type")) {
                "call-offer" -> {
                    if (state.value != State.IDLE) {
                        // Busy: auto-reject.
                        sendSignal?.invoke(from, JSONObject().put("type", "call-reject"))
                        return
                    }
                    peerPub.value = from
                    pendingOfferSdp = sig.optString("sdp").ifEmpty { null }
                    pendingVideo = sig.optBoolean("video", false)
                    onMain { state.value = State.INCOMING }
                }
                "call-answer" -> {
                    if (peerPub.value == from) {
                        val sdp = sig.optString("sdp") ?: return
                        Thread {
                            pc?.setRemoteDescription(object : SdpObserver {
                                override fun onSetSuccess() { onMain { state.value = State.CONNECTED } }
                                override fun onSetFailure(e: String) { onMain { fail(e) } }
                                override fun onCreateSuccess(s: SessionDescription) {}
                                override fun onCreateFailure(e: String) {}
                            }, SessionDescription(SessionDescription.Type.ANSWER, sdp))
                        }.start()
                    }
                }
                "call-ice" -> {
                    val c = sig.optJSONObject("candidate") ?: return
                    val cand = IceCandidate(
                        c.optString("sdpMid"),
                        c.optInt("sdpMLineIndex"),
                        c.optString("candidate")
                    )
                    Thread { pc?.addIceCandidate(cand) }.start()
                }
                "call-reject", "call-end" -> {
                    if (peerPub.value == from || peerPub.value.isEmpty()) {
                        onMain { endCallLocal(notify = false) }
                    }
                }
            }
        } catch (e: Exception) {
            Log.e("CallManager", "bad call signal", e)
        }
    }

    fun attachRemoteVideo(renderer: SurfaceViewRenderer, ctx: Context) {
        init(ctx)
        renderer.init(eglBase!!.eglBaseContext, null)
        remoteVideoTrack?.addSink(renderer)
    }
    fun attachLocalVideo(renderer: SurfaceViewRenderer, ctx: Context) {
        init(ctx)
        renderer.init(eglBase!!.eglBaseContext, null)
        localVideoTrack?.addSink(renderer)
    }
}
