import { useCallback, useEffect, useRef, useState } from 'react';
import { relayBaseURLWS } from '../config/endpoints';

export interface WSMessage {
  type: string;
  data: unknown;
}

export interface CallParticipant {
  id: string;
  display_name: string;
  stream_id: string;
  is_audio_on: boolean;
  is_video_on: boolean;
}

export interface CallRoomState {
  room_id: string;
  participants: CallParticipant[];
}

export interface VoiceCredentials {
  turn_url: string;
  turn_username: string;
  turn_credential: string;
  sfu_token: string;
  channel_uuid: string;
}

export interface UseWebSocketReturn {
  isConnected: boolean;
  participantId: string | null;
  participants: CallParticipant[];
  voiceCredentials: VoiceCredentials | null;
  timeRemaining: number | null;
  roomTier: string | null;
  maxDuration: number | null;
  timeWarning: boolean;
  timeExpired: boolean;
  joinRoom: (roomId: string, displayName: string, streamId: string, token?: string) => void;
  leaveRoom: (roomId: string) => void;
  updateMedia: (roomId: string, isAudioOn: boolean, isVideoOn: boolean) => void;
  updateStreamId: (roomId: string, streamId: string) => void;
}

export function useWebSocket(): UseWebSocketReturn {
  const [isConnected, setIsConnected] = useState(false);
  const [participantId, setParticipantId] = useState<string | null>(null);
  const [participants, setParticipants] = useState<CallParticipant[]>([]);
  const [voiceCredentials, setVoiceCredentials] = useState<VoiceCredentials | null>(null);
  const [timeRemaining, setTimeRemaining] = useState<number | null>(null); // seconds
  const [roomTier, setRoomTier] = useState<string | null>(null);
  const [maxDuration, setMaxDuration] = useState<number | null>(null); // seconds
  const [timeWarning, setTimeWarning] = useState(false);
  const [timeExpired, setTimeExpired] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const currentRoomRef = useRef<string | null>(null);

  useEffect(() => {
    const ws = new WebSocket(relayBaseURLWS);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
      setIsConnected(true);
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setIsConnected(false);
      setParticipantId(null);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        handleMessage(msg);
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err);
      }
    };

    return () => {
      ws.close();
    };
  }, []);

  const handleMessage = useCallback((msg: WSMessage) => {
    console.log('WS message:', msg.type, msg.data);

    switch (msg.type) {
      case 'call_room_state': {
        const state = msg.data as CallRoomState;
        setParticipants(state.participants);
        break;
      }
      case 'call_room_joined': {
        const data = msg.data as { room_id: string; participant_id: string };
        setParticipantId(data.participant_id);
        break;
      }
      case 'call_participant_joined': {
        const data = msg.data as { room_id: string; participant: CallParticipant };
        setParticipants(prev => [...prev, data.participant]);
        break;
      }
      case 'call_participant_left': {
        const data = msg.data as { room_id: string; participant_id: string };
        setParticipants(prev => prev.filter(p => p.id !== data.participant_id));
        break;
      }
      case 'call_media_updated': {
        const data = msg.data as {
          room_id: string;
          participant_id: string;
          is_audio_on: boolean;
          is_video_on: boolean;
        };
        setParticipants(prev =>
          prev.map(p =>
            p.id === data.participant_id
              ? { ...p, is_audio_on: data.is_audio_on, is_video_on: data.is_video_on }
              : p
          )
        );
        break;
      }
      case 'call_stream_id_updated': {
        const data = msg.data as {
          room_id: string;
          participant_id: string;
          stream_id: string;
        };
        setParticipants(prev =>
          prev.map(p =>
            p.id === data.participant_id
              ? { ...p, stream_id: data.stream_id }
              : p
          )
        );
        break;
      }
      case 'voice_credentials': {
        const creds = msg.data as VoiceCredentials;
        console.log('Received voice credentials for room:', creds.channel_uuid);
        setVoiceCredentials(creds);
        break;
      }
      case 'call_time_remaining': {
        const data = msg.data as { room_id: string; seconds_left: number; tier: string; max_duration_sec: number };
        setTimeRemaining(data.seconds_left);
        setRoomTier(data.tier);
        setMaxDuration(data.max_duration_sec);
        break;
      }
      case 'call_time_warning': {
        setTimeWarning(true);
        const data = msg.data as { seconds_left: number };
        setTimeRemaining(data.seconds_left);
        break;
      }
      case 'call_time_expired': {
        setTimeExpired(true);
        break;
      }
      case 'error': {
        const data = msg.data as { error?: string; content?: string };
        console.error('Server error:', data.error || data.content);
        break;
      }
    }
  }, []);

  const sendMessage = useCallback((msg: WSMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  const joinRoom = useCallback((roomId: string, displayName: string, streamId: string, token?: string) => {
    currentRoomRef.current = roomId;
    sendMessage({
      type: 'join_call_room',
      data: {
        room_id: roomId,
        display_name: displayName,
        stream_id: streamId,
        ...(token && { token }),
      },
    });
  }, [sendMessage]);

  const leaveRoom = useCallback((roomId: string) => {
    sendMessage({
      type: 'leave_call_room',
      data: {
        room_id: roomId,
      },
    });
    currentRoomRef.current = null;
    setParticipants([]);
    setParticipantId(null);
    setVoiceCredentials(null);
    setTimeRemaining(null);
    setRoomTier(null);
    setMaxDuration(null);
    setTimeWarning(false);
    setTimeExpired(false);
  }, [sendMessage]);

  const updateMedia = useCallback((roomId: string, isAudioOn: boolean, isVideoOn: boolean) => {
    sendMessage({
      type: 'update_call_media',
      data: {
        room_id: roomId,
        is_audio_on: isAudioOn,
        is_video_on: isVideoOn,
      },
    });
  }, [sendMessage]);

  const updateStreamId = useCallback((roomId: string, streamId: string) => {
    sendMessage({
      type: 'update_call_stream_id',
      data: {
        room_id: roomId,
        stream_id: streamId,
      },
    });
  }, [sendMessage]);

  return {
    isConnected,
    participantId,
    participants,
    voiceCredentials,
    timeRemaining,
    roomTier,
    maxDuration,
    timeWarning,
    timeExpired,
    joinRoom,
    leaveRoom,
    updateMedia,
    updateStreamId,
  };
}
