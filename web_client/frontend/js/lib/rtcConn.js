import { Client, LocalStream, RemoteStream } from "ion-sdk-js";
import { IonSFUJSONRPCSignal } from "ion-sdk-js/lib/signal/json-rpc-impl";

export default class RTCConnUsingIon {
  constructor(props) {
    this.audioCtx = props.audioCtx;
    this.room = props.room;
    this.userID = props.userID;

    this.sfuUrl = "ws://99.36.161.96:7000/ws"; // Update this as needed

    this.signal = null;
    this.client = null;
    this.audioElement = null;

    this.peerConfig = {
      iceServers: [
        {
          urls: "turn:99.36.161.96:3478?transport=udp",
          username: "1747510821",
          credential: "9U9t8QqEdHbKF71Fv4sU9GmN0vw",
        },
        {
          urls: "stun:stun.l.google.com:19302",
        },
      ],
      iceTransportPolicy: "all",
    };
  }

  async start() {
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

      if (!this.audioCtx || this.audioCtx.state === "closed") {
        this.audioCtx = new (window.AudioContext ||
          window.webkitAudioContext)();
        this.audioCtx.resume();
      }

      const source = this.audioCtx.createMediaStreamSource(stream);
      const gainNode = this.audioCtx.createGain();
      gainNode.gain.value = 1.0;
      source.connect(gainNode).connect(this.audioCtx.destination);
    };

    this.signal.onopen = async () => {
      console.log("🔗 Signal connection open, joining room:", this.room);
      await this.client.join(this.room, String(this.userID));

      // Using LocalStream to get the local media
      const localStream = await LocalStream.getUserMedia({
        audio: true,
        video: true,
      }); // Request audio/video
      this.client.publish(localStream); // Publish local stream using ion-sdk-js's client
      console.log("📤 Local stream published");

      // Optional: Simulcast setup if required for video
      // await this.client.publish(localStream, { simulcast: true });
    };
  }

  async close() {
    if (this.client) {
      await this.client.close();
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

    if (this.audioCtx) {
      this.audioCtx.close();
      this.audioCtx = null;
    }

    console.log("🔌 RTC connection closed");
  }
}
