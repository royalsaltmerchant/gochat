import { useState, useEffect, useCallback, useRef } from 'react';
import { VideoGrid } from './VideoGrid';
import { ControlBar } from './ControlBar';
import { ShareLink } from './ShareLink';
import { PreJoinPreview } from './PreJoinPreview';
import { useLocalStream } from '../hooks/useLocalStream';
import { useWebSocket } from '../hooks/useWebSocket';
import { useRTCConnection } from '../hooks/useRTCConnection';

interface CallRoomProps {
  roomId: string;
}

type CallState = 'preview' | 'joining' | 'connected' | 'ended';

export function CallRoom({ roomId }: CallRoomProps) {
  const [callState, setCallState] = useState<CallState>('preview');
  const [displayName, setDisplayName] = useState('');
  const [showShareLink, setShowShareLink] = useState(false);
  const pendingJoinRef = useRef<{ name: string; streamId: string } | null>(null);

  // Separate state for when connected (RTC controls the actual stream)
  const [connectedAudioEnabled, setConnectedAudioEnabled] = useState(true);
  const [connectedVideoEnabled, setConnectedVideoEnabled] = useState(true);

  const {
    localStream: previewStream,
    isAudioEnabled: previewAudioEnabled,
    isVideoEnabled: previewVideoEnabled,
    error: mediaError,
    startStream,
    stopStream,
    toggleAudio,
    toggleVideo,
  } = useLocalStream();

  // Use preview state when in preview, connected state when connected
  const isAudioEnabled = callState === 'connected' ? connectedAudioEnabled : previewAudioEnabled;
  const isVideoEnabled = callState === 'connected' ? connectedVideoEnabled : previewVideoEnabled;

  const {
    isConnected: wsConnected,
    participants,
    voiceCredentials,
    joinRoom,
    leaveRoom,
    updateMedia,
  } = useWebSocket();

  const {
    connectionState,
    localStream: rtcLocalStream,
    remoteStreams,
    connect: connectRTC,
    disconnect: disconnectRTC,
    setAudioEnabled,
    setVideoEnabled,
  } = useRTCConnection();

  // Start preview stream on mount
  useEffect(() => {
    startStream();
    return () => {
      stopStream();
    };
  }, [startStream, stopStream]);

  // Handle joining when voice credentials are received
  useEffect(() => {
    if (voiceCredentials && pendingJoinRef.current && callState === 'joining') {
      const { name } = pendingJoinRef.current;
      pendingJoinRef.current = null;

      // Initialize connected state from preview state
      setConnectedAudioEnabled(previewAudioEnabled);
      setConnectedVideoEnabled(previewVideoEnabled);

      // Now connect RTC with the credentials
      connectRTC(roomId, name, voiceCredentials, previewStream || undefined)
        .then((rtcStreamId) => {
          if (rtcStreamId) {
            console.log('RTC connected with stream:', rtcStreamId);
            setCallState('connected');
          } else {
            console.error('Failed to connect RTC');
            setCallState('preview');
          }
        })
        .catch((err) => {
          console.error('RTC connection error:', err);
          setCallState('preview');
        });
    }
  }, [voiceCredentials, callState, roomId, previewStream, previewAudioEnabled, previewVideoEnabled, connectRTC]);

  const handleJoin = useCallback((name: string) => {
    if (!wsConnected) {
      console.error('WebSocket not connected');
      return;
    }

    setDisplayName(name);
    setCallState('joining');

    // Generate a temporary stream ID for the WebSocket join
    // The actual RTC stream ID will be used after connection
    const tempStreamId = `pending-${Date.now()}`;
    pendingJoinRef.current = { name, streamId: tempStreamId };

    // Join WebSocket room first - this triggers voice_credentials to be sent
    joinRoom(roomId, name, tempStreamId);
  }, [roomId, wsConnected, joinRoom]);

  const handleLeave = useCallback(() => {
    leaveRoom(roomId);
    disconnectRTC();
    setCallState('ended');
  }, [roomId, leaveRoom, disconnectRTC]);

  const handleToggleAudio = useCallback(async () => {
    const newState = !isAudioEnabled;
    if (callState === 'connected') {
      // When connected, toggle via RTC service and update local state
      setConnectedAudioEnabled(newState);
      await setAudioEnabled(newState);
      updateMedia(roomId, newState, isVideoEnabled);
    } else {
      // In preview mode, toggle the preview stream
      toggleAudio();
    }
  }, [toggleAudio, setAudioEnabled, isAudioEnabled, isVideoEnabled, callState, roomId, updateMedia]);

  const handleToggleVideo = useCallback(async () => {
    const newState = !isVideoEnabled;
    if (callState === 'connected') {
      // When connected, toggle via RTC service and update local state
      setConnectedVideoEnabled(newState);
      await setVideoEnabled(newState);
      updateMedia(roomId, isAudioEnabled, newState);
    } else {
      // In preview mode, toggle the preview stream
      toggleVideo();
    }
  }, [toggleVideo, setVideoEnabled, isAudioEnabled, isVideoEnabled, callState, roomId, updateMedia]);

  // Show preview state
  if (callState === 'preview' || callState === 'joining') {
    return (
      <PreJoinPreview
        localStream={previewStream}
        isAudioOn={isAudioEnabled}
        isVideoOn={isVideoEnabled}
        onToggleAudio={handleToggleAudio}
        onToggleVideo={handleToggleVideo}
        onJoin={handleJoin}
        error={mediaError}
      />
    );
  }

  // Show ended state
  if (callState === 'ended') {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center">
          <h1 className="text-3xl font-bold text-white mb-4">Call ended</h1>
          <p className="text-gray-400 mb-6">You have left the call</p>
          <button
            onClick={() => window.close()}
            className="bg-blue-600 hover:bg-blue-700 text-white font-semibold py-3 px-6 rounded-xl transition-all duration-200"
          >
            Close window
          </button>
        </div>
      </div>
    );
  }

  // Show connected state
  return (
    <div className="h-screen flex flex-col bg-gray-900">
      {/* Connection status indicator */}
      {connectionState !== 'connected' && (
        <div className="bg-yellow-500/20 text-yellow-400 px-4 py-2 text-center text-sm">
          {connectionState === 'connecting' ? 'Connecting...' : 'Connection issue'}
        </div>
      )}

      {/* Video grid area */}
      <div className="flex-1 overflow-hidden">
        <VideoGrid
          localStream={rtcLocalStream || previewStream}
          localDisplayName={displayName}
          localIsAudioOn={isAudioEnabled}
          localIsVideoOn={isVideoEnabled}
          participants={participants}
          remoteStreams={remoteStreams}
        />
      </div>

      {/* Control bar */}
      <ControlBar
        isAudioOn={isAudioEnabled}
        isVideoOn={isVideoEnabled}
        onToggleAudio={handleToggleAudio}
        onToggleVideo={handleToggleVideo}
        onLeave={handleLeave}
        onShareLink={() => setShowShareLink(true)}
      />

      {/* Share link modal */}
      {showShareLink && (
        <ShareLink roomId={roomId} onClose={() => setShowShareLink(false)} />
      )}
    </div>
  );
}
