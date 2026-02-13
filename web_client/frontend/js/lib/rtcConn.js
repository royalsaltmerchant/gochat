import { Client, LocalStream, RemoteStream } from "ion-sdk-js";
import { IonSFUJSONRPCSignal } from "ion-sdk-js/lib/signal/json-rpc-impl";
import { relayBaseURL, sfuBaseURLWS } from "./config.js";
import voiceManager from "./voiceManager.js";
import platform from "../platform/index.js";

export default class RTCConnUsingIon {
  constructor(props) {
    this.socketConn = props.socketConn;
    this.room = props.room;
    this.userID = props.userID;

    this.sfuUrl = sfuBaseURLWS;
    this.peerConfig = null;

    this.signal = null;
    this.client = null;
    this.localStream = null;
  }

  init = async () => {
    try {
      const token = await platform.loadAuthToken();
      const headers = {};

      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }

      const res = await fetch(`${relayBaseURL}/api/turn_credentials`, {
        headers,
      });

      const data = await res.json().catch(() => ({}));

      if (!res.ok) {
        throw new Error(
          data.error || "Failed to fetch credentials for TURN server"
        );
      }

      this.peerConfig = {
        iceServers: [
          {
            urls: data.url,
            username: data.username,
            credential: data.credential,
            credentialType: "password",
          },
          {
            urls: "stun:stun.l.google.com:19302",
          },
        ],
        iceTransportPolicy: "all",
      };

      console.log(this.peerConfig);
    } catch (err) {
      console.log(err);
      platform.alert(
        err?.message || "Failed to fetch credentials for TURN server"
      );
    }
  };

  start = async () => {
    // Initialize the signaling connection
    this.signal = new IonSFUJSONRPCSignal(this.sfuUrl);
    this.client = new Client(this.signal, this.peerConfig);

    // Handle remote track reception
    this.client.ontrack = (track, stream) => {
      console.log("🎧 Remote track received:", track.kind);
      voiceManager.addRemoteStream(stream);
    };

    this.signal.onopen = async () => {
      console.log("🔗 Signal connection open, joining room:", this.room);
      await this.client.join(this.room, String(this.userID));

      // Using LocalStream to get the local media
      this.localStream = await LocalStream.getUserMedia({
        audio: true,
        video: false,
      }); // Request audio/video
      this.client.publish(this.localStream); // Publish local stream using ion-sdk-js's client
      console.log("📤 Local stream published", this.localStream);
      // Send local stream ID with user ID so we can find their info later
      this.socketConn.joinVoiceChannel(this.localStream.id);

      // Optional: Simulcast setup if required for video
      // await this.client.publish(localStream, { simulcast: true });
    };
  };

  close = async () => {
    if (this.client) {
      this.client.close();
      this.client = null;
    }

    if (this.signal) {
      this.signal.close();
      this.signal = null;
    }

    console.log("🔌 RTC connection closed");
  };
}
