import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { VideoGrid } from './VideoGrid';
import { ControlBar } from './ControlBar';
import { ShareLink } from './ShareLink';
import { PreJoinPreview } from './PreJoinPreview';
import { useLocalStream } from '../hooks/useLocalStream';
import { useWebSocket } from '../hooks/useWebSocket';
import { useRTCConnection } from '../hooks/useRTCConnection';
import {
  AUDIO_DEVICE_STORAGE_KEY,
  VIDEO_DEVICE_STORAGE_KEY,
  setStoredDeviceId,
} from '../config/mediaStorage';
import { useAuth } from '../contexts/AuthContext';

interface CallRoomProps {
  roomId: string;
}

type CallState = 'preview' | 'joining' | 'connected' | 'ended';

function formatTime(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
  return `${m}:${String(s).padStart(2, '0')}`;
}

export function CallRoom({ roomId }: CallRoomProps) {
  const [callState, setCallState] = useState<CallState>('preview');
  const [displayName, setDisplayName] = useState('');
  const [showShareLink, setShowShareLink] = useState(false);
  const [displayTime, setDisplayTime] = useState<number | null>(null);
  const [showTimeWarningModal, setShowTimeWarningModal] = useState(false);
  const warningShownRef = useRef(false);
  const { token } = useAuth();
  const pendingJoinRef = useRef<{ name: string; streamId: string } | null>(null);
  const hideControlsTimeoutRef = useRef<number | null>(null);
  const [showControls, setShowControls] = useState(true);

  // Separate state for when connected (RTC controls the actual stream)
  const [connectedAudioEnabled, setConnectedAudioEnabled] = useState(true);
  const [connectedVideoEnabled, setConnectedVideoEnabled] = useState(true);
  const [connectedSelectedAudioDeviceId, setConnectedSelectedAudioDeviceId] = useState<string | null>(null);
  const [connectedSelectedVideoDeviceId, setConnectedSelectedVideoDeviceId] = useState<string | null>(null);

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
    timeRemaining,
    roomTier,
    timeWarning,
    timeExpired,
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
    switchAudioDevice,
    switchVideoDevice,
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

  // Countdown timer synced with server time
  useEffect(() => {
    if (timeRemaining === null) return;
    setDisplayTime(timeRemaining);

    const interval = setInterval(() => {
      setDisplayTime(prev => {
        if (prev === null || prev <= 0) return 0;
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(interval);
  }, [timeRemaining]);

  // Show warning modal once when timeWarning triggers
  useEffect(() => {
    if (timeWarning && !warningShownRef.current) {
      warningShownRef.current = true;
      setShowTimeWarningModal(true);
    }
  }, [timeWarning]);

  // Handle joining when voice credentials are received
  useEffect(() => {
    if (voiceCredentials && pendingJoinRef.current && callState === 'joining') {
      const { name } = pendingJoinRef.current;
      pendingJoinRef.current = null;

      // Initialize connected state from preview state
      setConnectedAudioEnabled(previewAudioEnabled);
      setConnectedVideoEnabled(previewVideoEnabled);
      setConnectedSelectedAudioDeviceId(selectedAudioDeviceId);
      setConnectedSelectedVideoDeviceId(selectedVideoDeviceId);

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
  }, [voiceCredentials, callState, roomId, previewStream, previewAudioEnabled, previewVideoEnabled, selectedAudioDeviceId, selectedVideoDeviceId, connectRTC, updateStreamId]);

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
    // Use token from context, or fall back to localStorage in case auth restore is still in-flight
    const authToken = token || localStorage.getItem('call_app:auth_token') || undefined;
    joinRoom(roomId, name, tempStreamId, authToken);
  }, [roomId, wsConnected, joinRoom, token]);

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

  const handleSelectAudioDevice = useCallback(
    async (deviceId: string) => {
      try {
        if (callState === 'connected') {
          await switchAudioDevice(deviceId);
          setConnectedSelectedAudioDeviceId(deviceId);
          setStoredDeviceId(AUDIO_DEVICE_STORAGE_KEY, deviceId);
          return;
        }
        await selectAudioDevice(deviceId);
      } catch (err) {
        console.error('Failed to switch audio device:', err);
      }
    },
    [callState, selectAudioDevice, switchAudioDevice]
  );

  const handleSelectVideoDevice = useCallback(
    async (deviceId: string) => {
      try {
        if (callState === 'connected') {
          await switchVideoDevice(deviceId);
          setConnectedSelectedVideoDeviceId(deviceId);
          setStoredDeviceId(VIDEO_DEVICE_STORAGE_KEY, deviceId);
          return;
        }
        await selectVideoDevice(deviceId);
      } catch (err) {
        console.error('Failed to switch video device:', err);
      }
    },
    [callState, selectVideoDevice, switchVideoDevice]
  );

  const activeSelectedAudioDeviceId =
    callState === 'connected'
      ? (connectedSelectedAudioDeviceId ?? selectedAudioDeviceId)
      : selectedAudioDeviceId;
  const activeSelectedVideoDeviceId =
    callState === 'connected'
      ? (connectedSelectedVideoDeviceId ?? selectedVideoDeviceId)
      : selectedVideoDeviceId;

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
        onSelectAudioDevice={handleSelectAudioDevice}
        onSelectVideoDevice={handleSelectVideoDevice}
      />
    );
  }

  // Show time expired state
  if (timeExpired) {
    return (
      <div className="min-h-screen flex items-center justify-center p-4">
        <div className="text-center max-w-md">
          <h1 className="text-2xl sm:text-3xl font-serif font-bold text-parch-bright-white mb-4 tracking-parch">
            Call time limit reached
          </h1>
          <p className="text-parch-gray mb-6 tracking-parch">
            Free calls are limited to 40 minutes. Upgrade for calls up to 6 hours.
          </p>
          <div className="flex gap-3 justify-center">
            <a
              href="/call/account"
              className="parch-btn bg-parch-light-blue text-parch-bright-white font-serif font-semibold py-3 px-6 rounded-lg transition-all duration-150 tracking-parch"
            >
              Upgrade
            </a>
            <button
              onClick={() => window.close()}
              className="parch-btn bg-parch-dark-blue text-parch-bright-white font-serif font-semibold py-3 px-6 rounded-lg transition-all duration-150 tracking-parch border border-parch-gray/30"
            >
              Close
            </button>
          </div>
        </div>
      </div>
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
          localAudioDeviceId={activeSelectedAudioDeviceId}
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
          audioDevices={audioDevices}
          videoDevices={videoDevices}
          selectedAudioDeviceId={activeSelectedAudioDeviceId}
          selectedVideoDeviceId={activeSelectedVideoDeviceId}
          onSelectAudioDevice={handleSelectAudioDevice}
          onSelectVideoDevice={handleSelectVideoDevice}
          displayTime={displayTime !== null ? formatTime(displayTime) : null}
          timeWarning={timeWarning}
          roomTier={roomTier}
        />
      </div>

      {/* Time warning modal */}
      {showTimeWarningModal && (
        <div className="absolute inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/60" onClick={() => setShowTimeWarningModal(false)} />
          <div className="relative parch-card rounded-xl p-6 sm:p-8 shadow-2xl max-w-sm w-full text-center">
            <div className="text-parch-light-red text-4xl mb-4">
              <svg className="w-12 h-12 mx-auto" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <h2 className="text-xl font-serif font-bold text-parch-bright-white mb-2 tracking-parch">
              Call ending soon
            </h2>
            <p className="text-parch-gray text-sm tracking-parch mb-5">
              {displayTime !== null ? formatTime(displayTime) : 'Less than 5 minutes'} remaining.
              {roomTier === 'free' ? ' Upgrade for calls up to 6 hours.' : ''}
            </p>
            <div className="flex gap-3 justify-center">
              {roomTier === 'free' && (
                <a
                  href="/call/pricing"
                  target="_blank"
                  className="parch-btn bg-parch-light-blue text-parch-bright-white font-serif font-semibold py-2.5 px-5 rounded-lg tracking-parch text-sm"
                >
                  View Plans
                </a>
              )}
              <button
                onClick={() => setShowTimeWarningModal(false)}
                className="parch-btn bg-parch-dark-blue text-parch-bright-white font-serif font-semibold py-2.5 px-5 rounded-lg tracking-parch text-sm border border-parch-gray/30"
              >
                Got it
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Share link modal */}
      {showShareLink && (
        <ShareLink roomId={roomId} onClose={() => setShowShareLink(false)} />
      )}
    </div>
  );
}
