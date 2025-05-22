import RTCConn from "./rtcConn.js";

class VoiceManager {
  constructor() {
    this.socketConn = null;
    this.currentRTCConn = null;
    this.audioCtx = null;

    this.onStreamAdded = null; // Setup by voiceControl
    this.onStreamRemoved = null; // Setup by voiceControl

    this.currentChannelUUID = null;
    this.remoteStreams = new Map();
    this.voiceSubscriptions = [];
  }

  initAudio = async () => {
    this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    await this.audioCtx.resume(); // <- this unlocks autoplay
  };

  joinVoice = async ({ channelUUID, userID }) => {
    if (!this.audioCtx) {
      throw new Error(
        "Audio context not initialized. Call initAudio() from a user gesture first."
      );
    }

    this.currentChannelUUID = channelUUID;

    this.currentRTCConn = new RTCConn({
      room: this.currentChannelUUID,
      userID: userID,
      socketConn: this.socketConn,
    });

    await this.currentRTCConn.init();
    await this.currentRTCConn.start(); // Connects to SFU and joins room, sets local stream
  };

  addRemoteStream = (stream) => {
    if (this.remoteStreams.has(stream.id)) return;

    const source = this.audioCtx.createMediaStreamSource(stream);
    const gainNode = this.audioCtx.createGain();
    source.connect(gainNode).connect(this.audioCtx.destination);

    stream.onremovetrack = (event) => {
      const stream = event.target;
      this.removeRemoteStream(stream.id);
    };

    this.remoteStreams.set(stream.id, {
      stream,
      source,
      gainNode,
    });

    if (this.onStreamAdded) {
      this.onStreamAdded(stream);
    }
  };

  removeRemoteStream = (streamID) => {
    const entry = this.remoteStreams.get(streamID);
    if (!entry) return;

    entry.source.disconnect();
    entry.gainNode.disconnect();
    this.remoteStreams.delete(streamID);

    if (this.onStreamRemoved) {
      this.onStreamRemoved(streamID);
    }
  };

  clearAll = () => {
    for (const id of this.remoteStreams.keys()) {
      this.removeRemoteStream(id);
    }
  };

  // Public: used by UI components
  getAudioElements = () => {
    return Array.from(this.remoteStreams.values()).map((v) => v.element);
  };

  leaveVoice = async () => {
    this.currentChannelUUID = null;

    if (this.currentRTCConn) {
      await this.currentRTCConn.close();
      this.currentRTCConn = null;
    }

    if (this.audioCtx) {
      await this.audioCtx.close();
      this.audioCtx = null;
    }

    // Send to relay
    this.socketConn.leaveVoiceChannel();
  };
}

const voiceManager = new VoiceManager();
export default voiceManager;
