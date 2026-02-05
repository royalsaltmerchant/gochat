import { Client, LocalStream, RemoteStream } from "ion-sdk-js";
import type { Constraints } from "ion-sdk-js/lib/stream";
import { Configuration } from "ion-sdk-js/lib/client";
import { IonSFUJSONRPCSignal } from "ion-sdk-js/lib/signal/json-rpc-impl";
import { sfuBaseURLWS } from "../config/endpoints";

export interface RTCServiceCallbacks {
  onRemoteStream: (stream: RemoteStream) => void;
  onRemoteStreamRemoved: (streamId: string) => void;
  onConnectionStateChange: (state: 'connecting' | 'connected' | 'disconnected' | 'failed') => void;
}

export interface VoiceCredentials {
  turn_url: string;
  turn_username: string;
  turn_credential: string;
  sfu_token: string;
  channel_uuid: string;
}

export default class RTCService {
  private sfuUrl: string;
  private peerConfig: Configuration | null = null;
  private signal: IonSFUJSONRPCSignal | null = null;
  private client: Client | null = null;
  private localStream: LocalStream | null = null;
  private callbacks: RTCServiceCallbacks;
  private room: string;
  private userId: string;

  constructor(room: string, userId: string, callbacks: RTCServiceCallbacks) {
    this.room = room;
    this.userId = userId;
    this.callbacks = callbacks;
    this.sfuUrl = sfuBaseURLWS;
  }

  /**
   * Initialize RTC config with voice credentials received from WebSocket
   */
  initWithCredentials(credentials: VoiceCredentials): void {
    // Include SFU token in the WebSocket URL for authentication
    this.sfuUrl = `${sfuBaseURLWS}?token=${encodeURIComponent(credentials.sfu_token)}`;

    this.peerConfig = {
      codec: 'vp9',
      iceServers: [
        {
          urls: credentials.turn_url,
          username: credentials.turn_username,
          credential: credentials.turn_credential,
        },
        {
          urls: "stun:stun.l.google.com:19302",
        },
      ],
      iceTransportPolicy: "all",
    };
    console.log("RTC config initialized with voice credentials");
  }

  async start(existingStream?: MediaStream): Promise<MediaStream> {
    this.callbacks.onConnectionStateChange('connecting');

    this.signal = new IonSFUJSONRPCSignal(this.sfuUrl);
    this.client = new Client(this.signal, this.peerConfig || undefined);

    this.client.ontrack = (track, stream) => {
      console.log("Remote track received:", track.kind, stream.id);
      this.callbacks.onRemoteStream(stream as RemoteStream);

      stream.onremovetrack = () => {
        console.log("Remote track removed:", stream.id);
        this.callbacks.onRemoteStreamRemoved(stream.id);
      };
    };

    return new Promise((resolve, reject) => {
      if (!this.signal) {
        reject(new Error("Signal not initialized"));
        return;
      }

      this.signal.onopen = async () => {
        console.log("Signal connection open, joining room:", this.room);
        try {
          await this.client!.join(this.room, this.userId);
          this.callbacks.onConnectionStateChange('connected');

          // Use existing stream or get new one with video
          if (existingStream) {
            // Create LocalStream from existing MediaStream
            const constraints: Constraints = {
              resolution: 'fhd',
              codec: 'vp9',
              audio: true,
              video: true,
            };
            this.localStream = new LocalStream(existingStream, constraints);
          } else {
            this.localStream = await LocalStream.getUserMedia({
              resolution: 'fhd',
              codec: 'vp9',
              audio: true,
              video: true,
            });
          }

          this.client!.publish(this.localStream);
          console.log("Local stream published:", this.localStream.id);
          resolve(this.localStream);
        } catch (err) {
          console.error("Error joining room:", err);
          this.callbacks.onConnectionStateChange('failed');
          reject(err);
        }
      };

      this.signal.onerror = (err) => {
        console.error("Signal error:", err);
        this.callbacks.onConnectionStateChange('failed');
        reject(err);
      };
    });
  }

  getLocalStream(): LocalStream | null {
    return this.localStream;
  }

  getLocalStreamId(): string | null {
    return this.localStream?.id || null;
  }

  async setAudioEnabled(enabled: boolean): Promise<void> {
    if (this.localStream) {
      if (enabled) {
        await this.localStream.unmute('audio');
      } else {
        this.localStream.mute('audio');
      }
    }
  }

  async setVideoEnabled(enabled: boolean): Promise<void> {
    if (this.localStream) {
      if (enabled) {
        await this.localStream.unmute('video');
      } else {
        this.localStream.mute('video');
      }
    }
  }

  async close(): Promise<void> {
    if (this.localStream) {
      this.localStream.getTracks().forEach(track => track.stop());
      this.localStream = null;
    }

    if (this.client) {
      this.client.close();
      this.client = null;
    }

    if (this.signal) {
      this.signal.close();
      this.signal = null;
    }

    this.callbacks.onConnectionStateChange('disconnected');
    console.log("RTC connection closed");
  }
}
