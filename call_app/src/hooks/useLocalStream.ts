import { useCallback, useEffect, useRef, useState } from "react";

export interface UseLocalStreamReturn {
  localStream: MediaStream | null;
  isAudioEnabled: boolean;
  isVideoEnabled: boolean;
  error: string | null;
  startStream: () => Promise<MediaStream | null>;
  stopStream: () => void;
  toggleAudio: () => void;
  toggleVideo: () => void;
}

export function useLocalStream(): UseLocalStreamReturn {
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [isAudioEnabled, setIsAudioEnabled] = useState(true);
  const [isVideoEnabled, setIsVideoEnabled] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const streamRef = useRef<MediaStream | null>(null);

  const startStream = useCallback(async (): Promise<MediaStream | null> => {
    try {
      setError(null);
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          sampleRate: 48000,

          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: {
          width: { ideal: 1280 },
          height: { ideal: 720 },
          frameRate: { ideal: 30 },
        },
      });

      streamRef.current = stream;
      setLocalStream(stream);
      setIsAudioEnabled(true);
      setIsVideoEnabled(true);
      return stream;
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to access camera/microphone";
      setError(message);
      console.error("Failed to get user media:", err);
      return null;
    }
  }, []);

  const stopStream = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
      setLocalStream(null);
    }
  }, []);

  const toggleAudio = useCallback(() => {
    if (streamRef.current) {
      const audioTracks = streamRef.current.getAudioTracks();
      const newState = !isAudioEnabled;
      audioTracks.forEach((track) => {
        track.enabled = newState;
      });
      setIsAudioEnabled(newState);
    }
  }, [isAudioEnabled]);

  const toggleVideo = useCallback(() => {
    if (streamRef.current) {
      const videoTracks = streamRef.current.getVideoTracks();
      const newState = !isVideoEnabled;
      videoTracks.forEach((track) => {
        track.enabled = newState;
      });
      setIsVideoEnabled(newState);
    }
  }, [isVideoEnabled]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (streamRef.current) {
        streamRef.current.getTracks().forEach((track) => track.stop());
      }
    };
  }, []);

  return {
    localStream,
    isAudioEnabled,
    isVideoEnabled,
    error,
    startStream,
    stopStream,
    toggleAudio,
    toggleVideo,
  };
}
