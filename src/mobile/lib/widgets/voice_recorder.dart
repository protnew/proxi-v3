import 'dart:async';
import 'package:flutter/material.dart';
import '../services/theme.dart';

enum RecordingState { idle, recording, stopped }

class VoiceRecorder extends StatefulWidget {
  final void Function(String path, double duration) onRecordComplete;
  const VoiceRecorder({super.key, required this.onRecordComplete});

  @override
  State<VoiceRecorder> createState() => _VoiceRecorderState();
}

class _VoiceRecorderState extends State<VoiceRecorder> {
  RecordingState _state = RecordingState.idle;
  Duration _duration = Duration.zero;
  Timer? _timer;

  void _startRecording() {
    setState(() {
      _state = RecordingState.recording;
      _duration = Duration.zero;
    });

    _timer = Timer.periodic(const Duration(seconds: 1), (_) {
      setState(() {
        _duration += const Duration(seconds: 1);
      });
    });

    // TODO: Initialize actual recording with `record` package
  }

  void _stopRecording() {
    _timer?.cancel();
    setState(() => _state = RecordingState.stopped);

    final seconds = _duration.inSeconds.toDouble();
    // TODO: Get actual file path from recorder
    widget.onRecordComplete('/tmp/voice_${DateTime.now().millisecondsSinceEpoch}.aac', seconds);

    setState(() => _state = RecordingState.idle);
  }

  void _cancelRecording() {
    _timer?.cancel();
    setState(() => _state = RecordingState.idle);
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_state == RecordingState.idle) {
      return IconButton(
        icon: const Icon(Icons.mic, color: ProxiTheme.muted),
        onPressed: _startRecording,
        tooltip: 'Record voice',
      );
    }

    return Container(
      height: 48,
      decoration: BoxDecoration(
        color: ProxiTheme.surfaceVariant,
        borderRadius: BorderRadius.circular(24),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          const SizedBox(width: 12),
          // Pulsing red dot
          TweenAnimationBuilder<double>(
            tween: Tween(begin: 0.6, end: 1.0),
            duration: const Duration(milliseconds: 600),
            builder: (context, value, child) {
              return Opacity(
                opacity: value,
                child: Container(
                  width: 10,
                  height: 10,
                  decoration: const BoxDecoration(
                    color: ProxiTheme.danger,
                    shape: BoxShape.circle,
                  ),
                ),
              );
            },
            onEnd: () {
              if (_state == RecordingState.recording) setState(() {});
            },
          ),
          const SizedBox(width: 10),
          Text(
            '${_duration.inMinutes.toString().padLeft(2, '0')}:${(_duration.inSeconds % 60).toString().padLeft(2, '0')}',
            style: const TextStyle(color: ProxiTheme.onBackground, fontSize: 14),
          ),
          const SizedBox(width: 8),
          // Waveform placeholder
          SizedBox(
            width: 80,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceEvenly,
              children: List.generate(
                12,
                (_) => Container(
                  width: 2,
                  height: 10 + (_duration.inMilliseconds % 20).toDouble(),
                  decoration: BoxDecoration(
                    color: ProxiTheme.primary,
                    borderRadius: BorderRadius.circular(1),
                  ),
                ),
              ),
            ),
          ),
          IconButton(
            icon: const Icon(Icons.delete_outline, color: ProxiTheme.danger, size: 20),
            onPressed: _cancelRecording,
            tooltip: 'Cancel',
          ),
          IconButton(
            icon: const Icon(Icons.send, color: ProxiTheme.primary, size: 20),
            onPressed: _stopRecording,
            tooltip: 'Send',
          ),
          const SizedBox(width: 4),
        ],
      ),
    );
  }
}
