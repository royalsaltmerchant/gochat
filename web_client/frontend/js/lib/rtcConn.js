import { Client, LocalStream, RemoteStream } from "ion-sdk-js";
import { IonSFUJSONRPCSignal } from "ion-sdk-js/lib/signal/json-rpc-impl";
import { relayBaseURL } from "./config.js";
import voiceManager from "./voiceManager.js";

export default class RTCConnUsingIon {
  constructor(props) {
    this.room = props.room;
    this.userID = props.userID;

    this.sfuUrl = "ws://99.36.161.96:7000/ws"; // Update this as needed
    this.peerConfig = null;

    this.signal = null;
    this.client = null;
    this.audioElement = null;

  }

  init = async () => {
    try {
      const res = await fetch(`${relayBaseURL}/api/turn_credentials`);
      const data = await res.json();
      if (res.ok) {
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
        console.log(this.peerConfig)
      } else throw new Error("Failed to fetch credentials for TURN server");
    } catch (err) {
      console.log(err);
      window.go.main.App.Alert("Failed to fetch credentials for TURN server");
    }
  };

  start = async () => {
    // Initialize the signaling connection
    this.signal = new IonSFUJSONRPCSignal(this.sfuUrl);
    this.client = new Client(this.signal, this.peerConfig); // Pass ICE config here

    // Handle remote track reception
    this.client.ontrack = (track, stream) => {
      console.log("🎧 Remote track received:", track.kind);

      if (!this.audioElement) {
        this.audioElement = document.createElement("audio");
        this.audioElement.autoplay = true;
        this.audioElement.controls = true;
        this.audioElement.muted = false;
        document.body.appendChild(this.audioElement);
      }
      // Using RemoteStream for handling remote media
      this.audioElement.srcObject = stream;

      const source = voiceManager.audioCtx.createMediaStreamSource(stream);
      const gainNode = voiceManager.audioCtx.createGain();
      gainNode.gain.value = 1.0;
      source.connect(gainNode).connect(voiceManager.audioCtx.destination);
    };

    this.signal.onopen = async () => {
      console.log("🔗 Signal connection open, joining room:", this.room);
      await this.client.join(this.room, String(this.userID));

      // Using LocalStream to get the local media
      const localStream = await LocalStream.getUserMedia({
        audio: true,
        video: false,
      }); // Request audio/video
      this.client.publish(localStream); // Publish local stream using ion-sdk-js's client
      console.log("📤 Local stream published");

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

    if (this.audioElement) {
      this.audioElement.srcObject = null;
      this.audioElement.remove();
      this.audioElement = null;
    }

    console.log("🔌 RTC connection closed");
  };
}
