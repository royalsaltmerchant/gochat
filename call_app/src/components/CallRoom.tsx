import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
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
  const hideControlsTimeoutRef = useRef<number | null>(null);
  const [showControls, setShowControls] = useState(true);

  // Separate state for when connected (RTC controls the actual stream)
  const [connectedAudioEnabled, setConnectedAudioEnabled] = useState(true);
  const [connectedVideoEnabled, setConnectedVideoEnabled] = useState(true);

  const {
    localStream: previewStream,
    isAudioEnabled: previewAudioEnabled,
    isVideoEnabled: previewVideoEnabled,
    error: mediaError,
    audioDevices,
    videoDevices,
    selectedAudioDeviceId,
    selectedVideoDeviceId,
    startStream,
    stopStream,
    toggleAudio,
    toggleVideo,
    selectAudioDevice,
    selectVideoDevice,
  } = useLocalStream();

  // Use preview state when in preview, connected state when connected
  const isAudioEnabled = callState === 'connected' ? connectedAudioEnabled : previewAudioEnabled;
  const isVideoEnabled = callState === 'connected' ? connectedVideoEnabled : previewVideoEnabled;

  const {
    isConnected: wsConnected,
    participantId,
    participants,
    voiceCredentials,
    joinRoom,
    leaveRoom,
    updateMedia,
    updateStreamId,
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

  // Filter out the local user from participants list (should not be included, but filter as safety)
  const remoteParticipants = useMemo(() => {
    if (!participantId) return participants;
    return participants.filter(p => p.id !== participantId);
  }, [participants, participantId]);

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
            // Update the server with the actual RTC stream ID so other participants can match streams
            updateStreamId(roomId, rtcStreamId);
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
  }, [voiceCredentials, callState, roomId, previewStream, previewAudioEnabled, previewVideoEnabled, connectRTC, updateStreamId]);

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

  const handleUserActivity = useCallback(() => {
    setShowControls(true);
    if (hideControlsTimeoutRef.current !== null) {
      window.clearTimeout(hideControlsTimeoutRef.current);
    }
    hideControlsTimeoutRef.current = window.setTimeout(() => {
      setShowControls(false);
    }, 3000);
  }, []);

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

  // Auto-hide controls while connected
  useEffect(() => {
    if (callState !== 'connected') return;
    handleUserActivity();
    return () => {
      if (hideControlsTimeoutRef.current !== null) {
        window.clearTimeout(hideControlsTimeoutRef.current);
      }
    };
  }, [callState, handleUserActivity]);

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
        isJoining={callState === 'joining'}
        audioDevices={audioDevices}
        videoDevices={videoDevices}
        selectedAudioDeviceId={selectedAudioDeviceId}
        selectedVideoDeviceId={selectedVideoDeviceId}
        onSelectAudioDevice={selectAudioDevice}
        onSelectVideoDevice={selectVideoDevice}
      />
    );
  }

  // Show ended state
  if (callState === 'ended') {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center">
          <h1 className="text-2xl sm:text-3xl font-serif font-bold text-parch-bright-white mb-4 tracking-parch">
            Call ended
          </h1>
          <p className="text-parch-gray mb-6 tracking-parch">You have left the call</p>
          <button
            onClick={() => window.close()}
            className="parch-btn bg-parch-light-blue text-parch-bright-white font-serif font-semibold py-3 px-6 rounded-lg transition-all duration-150 tracking-parch"
          >
            Close window
          </button>
        </div>
      </div>
    );
  }

  // Show connected state
  return (
    <div
      className="h-screen flex flex-col bg-parch-dark relative overflow-hidden"
      onMouseMove={handleUserActivity}
      onTouchStart={handleUserActivity}
    >
      {/* Connection status indicator */}
      {connectionState !== 'connected' && (
        <div className="bg-parch-yellow/15 text-parch-yellow px-4 py-2 text-center text-sm tracking-parch border-b border-parch-yellow/20">
          {connectionState === 'connecting' ? 'Connecting...' : 'Connection issue'}
        </div>
      )}

      {/* Video grid area */}
      <div className="flex-1 min-h-0">
        <VideoGrid
          localStream={rtcLocalStream || previewStream}
          localDisplayName={displayName}
          localIsAudioOn={isAudioEnabled}
          localIsVideoOn={isVideoEnabled}
          participants={remoteParticipants}
          remoteStreams={remoteStreams}
        />
      </div>

      {/* Control bar (auto-hides) */}
      <div
        className={`absolute bottom-0 left-0 right-0 transition-opacity duration-200 ${
          showControls ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
      >
        <ControlBar
          isAudioOn={isAudioEnabled}
          isVideoOn={isVideoEnabled}
          onToggleAudio={handleToggleAudio}
          onToggleVideo={handleToggleVideo}
          onLeave={handleLeave}
          onShareLink={() => setShowShareLink(true)}
        />
      </div>

      {/* Share link modal */}
      {showShareLink && (
        <ShareLink roomId={roomId} onClose={() => setShowShareLink(false)} />
      )}
    </div>
  );
}
