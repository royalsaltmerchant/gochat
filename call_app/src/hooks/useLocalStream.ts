import { useCallback, useEffect, useRef, useState } from "react";
import {
  AUDIO_DEVICE_STORAGE_KEY,
  VIDEO_DEVICE_STORAGE_KEY,
  getStoredDeviceId,
  setStoredDeviceId,
} from "../config/mediaStorage";

export interface MediaDeviceInfo {
  deviceId: string;
  label: string;
  kind: "audioinput" | "videoinput";
}

export interface UseLocalStreamReturn {
  localStream: MediaStream | null;
  isAudioEnabled: boolean;
  isVideoEnabled: boolean;
  error: string | null;
  audioDevices: MediaDeviceInfo[];
  videoDevices: MediaDeviceInfo[];
  selectedAudioDeviceId: string | null;
  selectedVideoDeviceId: string | null;
  startStream: () => Promise<MediaStream | null>;
  stopStream: () => void;
  toggleAudio: () => void;
  toggleVideo: () => void;
  selectAudioDevice: (deviceId: string) => Promise<void>;
  selectVideoDevice: (deviceId: string) => Promise<void>;
}

export function useLocalStream(): UseLocalStreamReturn {
  const [localStream, setLocalStream] = useState<MediaStream | null>(null);
  const [isAudioEnabled, setIsAudioEnabled] = useState(true);
  const [isVideoEnabled, setIsVideoEnabled] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [audioDevices, setAudioDevices] = useState<MediaDeviceInfo[]>([]);
  const [videoDevices, setVideoDevices] = useState<MediaDeviceInfo[]>([]);
  const [selectedAudioDeviceId, setSelectedAudioDeviceId] = useState<string | null>(() =>
    getStoredDeviceId(AUDIO_DEVICE_STORAGE_KEY)
  );
  const [selectedVideoDeviceId, setSelectedVideoDeviceId] = useState<string | null>(() =>
    getStoredDeviceId(VIDEO_DEVICE_STORAGE_KEY)
  );
  const streamRef = useRef<MediaStream | null>(null);

  const enumerateDevices = useCallback(async () => {
    try {
      const devices = await navigator.mediaDevices.enumerateDevices();
      const audio: MediaDeviceInfo[] = [];
      const video: MediaDeviceInfo[] = [];

      devices.forEach((device, index) => {
        if (device.kind === "audioinput") {
          audio.push({
            deviceId: device.deviceId,
            label: device.label || `Microphone ${index + 1}`,
            kind: "audioinput",
          });
        } else if (device.kind === "videoinput") {
          video.push({
            deviceId: device.deviceId,
            label: device.label || `Camera ${index + 1}`,
            kind: "videoinput",
          });
        }
      });

      setAudioDevices(audio);
      setVideoDevices(video);

      setSelectedAudioDeviceId((prev) => {
        if (prev && audio.some((d) => d.deviceId === prev)) return prev;
        return audio[0]?.deviceId ?? null;
      });
      setSelectedVideoDeviceId((prev) => {
        if (prev && video.some((d) => d.deviceId === prev)) return prev;
        return video[0]?.deviceId ?? null;
      });
    } catch (err) {
      console.error("Failed to enumerate devices:", err);
    }
  }, []);

  const getStreamWithDevices = useCallback(
    async (audioDeviceId?: string, videoDeviceId?: string): Promise<MediaStream | null> => {
      const constraintsFor = (audioId?: string, videoId?: string): MediaStreamConstraints => ({
        audio: {
          deviceId: audioId ? { exact: audioId } : undefined,
          channelCount: 1,
          sampleRate: 48000,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: {
          deviceId: videoId ? { exact: videoId } : undefined,
          width: { ideal: 1920 },
          height: { ideal: 1080 },
          frameRate: { ideal: 30 },
        },
      });

      try {
        setError(null);
        let stream: MediaStream;
        try {
          stream = await navigator.mediaDevices.getUserMedia(
            constraintsFor(audioDeviceId, videoDeviceId)
          );
        } catch (firstErr) {
          // Saved device may no longer exist; retry with browser default devices.
          if (!audioDeviceId && !videoDeviceId) {
            throw firstErr;
          }
          console.warn("Saved media device unavailable, falling back to defaults:", firstErr);
          stream = await navigator.mediaDevices.getUserMedia(constraintsFor());
        }

        // Get actual device IDs from the stream tracks
        const audioTrack = stream.getAudioTracks()[0];
        const videoTrack = stream.getVideoTracks()[0];

        if (audioTrack) {
          const settings = audioTrack.getSettings();
          if (settings.deviceId) {
            setSelectedAudioDeviceId(settings.deviceId);
          }
        }

        if (videoTrack) {
          const settings = videoTrack.getSettings();
          if (settings.deviceId) {
            setSelectedVideoDeviceId(settings.deviceId);
          }
        }

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
    },
    []
  );

  const startStream = useCallback(async (): Promise<MediaStream | null> => {
    const stream = await getStreamWithDevices(
      selectedAudioDeviceId || undefined,
      selectedVideoDeviceId || undefined
    );
    if (stream) {
      streamRef.current = stream;
      setLocalStream(stream);
      setIsAudioEnabled(true);
      setIsVideoEnabled(true);
      // Enumerate devices after getting permission (labels become available)
      await enumerateDevices();
    }
    return stream;
  }, [getStreamWithDevices, enumerateDevices, selectedAudioDeviceId, selectedVideoDeviceId]);

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

  const selectAudioDevice = useCallback(
    async (deviceId: string) => {
      if (!streamRef.current) return;

      // Get new audio track with selected device
      const newStream = await getStreamWithDevices(deviceId, selectedVideoDeviceId || undefined);
      if (!newStream) return;

      // Stop old audio tracks
      streamRef.current.getAudioTracks().forEach((track) => track.stop());

      // Replace audio track in the stream
      const newAudioTrack = newStream.getAudioTracks()[0];
      if (newAudioTrack) {
        // Remove old audio tracks and add new one
        streamRef.current.getAudioTracks().forEach((track) => {
          streamRef.current?.removeTrack(track);
        });
        streamRef.current.addTrack(newAudioTrack);

        // Stop the video track from newStream since we don't need it
        newStream.getVideoTracks().forEach((track) => track.stop());

        // Apply current enabled state
        newAudioTrack.enabled = isAudioEnabled;
        setSelectedAudioDeviceId(deviceId);
        setLocalStream(streamRef.current);
      }
    },
    [getStreamWithDevices, selectedVideoDeviceId, isAudioEnabled]
  );

  const selectVideoDevice = useCallback(
    async (deviceId: string) => {
      if (!streamRef.current) return;

      // Get new video track with selected device
      const newStream = await getStreamWithDevices(selectedAudioDeviceId || undefined, deviceId);
      if (!newStream) return;

      // Stop old video tracks
      streamRef.current.getVideoTracks().forEach((track) => track.stop());

      // Replace video track in the stream
      const newVideoTrack = newStream.getVideoTracks()[0];
      if (newVideoTrack) {
        // Remove old video tracks and add new one
        streamRef.current.getVideoTracks().forEach((track) => {
          streamRef.current?.removeTrack(track);
        });
        streamRef.current.addTrack(newVideoTrack);

        // Stop the audio track from newStream since we don't need it
        newStream.getAudioTracks().forEach((track) => track.stop());

        // Apply current enabled state
        newVideoTrack.enabled = isVideoEnabled;
        setSelectedVideoDeviceId(deviceId);
        setLocalStream(streamRef.current);
      }
    },
    [getStreamWithDevices, selectedAudioDeviceId, isVideoEnabled]
  );

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (streamRef.current) {
        streamRef.current.getTracks().forEach((track) => track.stop());
      }
    };
  }, []);

  useEffect(() => {
    setStoredDeviceId(AUDIO_DEVICE_STORAGE_KEY, selectedAudioDeviceId);
  }, [selectedAudioDeviceId]);

  useEffect(() => {
    setStoredDeviceId(VIDEO_DEVICE_STORAGE_KEY, selectedVideoDeviceId);
  }, [selectedVideoDeviceId]);

  return {
    localStream,
    isAudioEnabled,
    isVideoEnabled,
    error,
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
  };
}
