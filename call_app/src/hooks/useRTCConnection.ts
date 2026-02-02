import { useCallback, useRef, useState } from 'react';
import RTCService, { VoiceCredentials } from '../services/rtcService';
import { RemoteStream } from 'ion-sdk-js';

export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'failed';

export interface RemoteStreamInfo {
  stream: MediaStream;
  streamId: string;
}

export interface UseRTCConnectionReturn {
  connectionState: ConnectionState;
  localStream: MediaStream | null;
  remoteStreams: Map<string, RemoteStreamInfo>;
  connect: (roomId: string, participantId: string, credentials: VoiceCredentials, existingStream?: MediaStream) => Promise<string | null>;
  disconnect: () => Promise<void>;
  setAudioEnabled: (enabled: boolean) => void;
  setVideoEnabled: (enabled: boolean) => void;
}

export function useRTCConnection(): UseRTCConnectionReturn {
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected');
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [remoteStreams, setRemoteStreams] = useState<Map<string, RemoteStreamInfo>>(new Map());
  const rtcServiceRef = useRef<RTCService | null>(null);

  const connect = useCallback(async (roomId: string, participantId: string, credentials: VoiceCredentials, existingStream?: MediaStream): Promise<string | null> => {
    if (rtcServiceRef.current) {
      await rtcServiceRef.current.close();
    }

    const rtcService = new RTCService(roomId, participantId, {
      onRemoteStream: (stream: RemoteStream) => {
        console.log('Remote stream received:', stream.id);
        setRemoteStreams(prev => {
          const newMap = new Map(prev);
          newMap.set(stream.id, { stream, streamId: stream.id });
          return newMap;
        });
      },
      onRemoteStreamRemoved: (streamId: string) => {
        console.log('Remote stream removed:', streamId);
        setRemoteStreams(prev => {
          const newMap = new Map(prev);
          newMap.delete(streamId);
          return newMap;
        });
      },
      onConnectionStateChange: (state) => {
        setConnectionState(state);
      },
    });

    rtcServiceRef.current = rtcService;

    try {
      // Initialize with voice credentials from WebSocket
      rtcService.initWithCredentials(credentials);
      const stream = await rtcService.start(existingStream);
      setLocalStream(stream);
      return rtcService.getLocalStreamId();
    } catch (err) {
      console.error('Failed to connect RTC:', err);
      setConnectionState('failed');
      return null;
    }
  }, []);

  const disconnect = useCallback(async () => {
    if (rtcServiceRef.current) {
      await rtcServiceRef.current.close();
      rtcServiceRef.current = null;
    }
    setLocalStream(null);
    setRemoteStreams(new Map());
    setConnectionState('disconnected');
  }, []);

  const setAudioEnabled = useCallback(async (enabled: boolean) => {
    await rtcServiceRef.current?.setAudioEnabled(enabled);
  }, []);

  const setVideoEnabled = useCallback(async (enabled: boolean) => {
    await rtcServiceRef.current?.setVideoEnabled(enabled);
  }, []);

  return {
    connectionState,
    localStream,
    remoteStreams,
    connect,
    disconnect,
    setAudioEnabled,
    setVideoEnabled,
  };
}
