import RTCConn from "./rtcConn.js";

class VoiceManager {
  constructor() {
    this.socketConn = null;
    this.currentRTCConn = null;

    this.onStreamAdded = null; // Setup by mediaControl
    this.onStreamRemoved = null; // Setup by mediaControl
    this.onLocalStream = null; // Callback when local stream is ready

    this.currentChannelUUID = null;
    this.remoteStreams = new Map();
    this.voiceSubscriptions = [];
    this.videoEnabled = false;
  }

  joinVoice = async ({ channelUUID, userID, enableVideo = false }) => {
    this.currentChannelUUID = channelUUID;
    this.videoEnabled = enableVideo;

    this.currentRTCConn = new RTCConn({
      room: this.currentChannelUUID,
      userID: userID,
      socketConn: this.socketConn,
      enableVideo: enableVideo,
      enableAudio: true,
      onLocalStream: (stream) => {
        // Notify UI of local stream for preview
        if (this.onLocalStream) {
          this.onLocalStream(stream);
        }
      },
    });

    await this.currentRTCConn.init();
    await this.currentRTCConn.start();
  };

  getLocalStream = () => {
    return this.currentRTCConn?.localStream || null;
  };

  isVideoEnabled = () => {
    return this.videoEnabled;
  };

  addRemoteStream = (stream) => {
    if (this.remoteStreams.has(stream.id)) return;

    stream.onremovetrack = (event) => {
      const stream = event.target;
      this.removeRemoteStream(stream.id);
    };

    this.remoteStreams.set(stream.id, {
      stream,
    });

    if (this.onStreamAdded) {
      this.onStreamAdded(stream);
    }
  };

  removeRemoteStream = (streamID) => {
    const entry = this.remoteStreams.get(streamID);
    if (!entry) return;

    this.remoteStreams.delete(streamID);

    if (this.onStreamRemoved) {
      this.onStreamRemoved(streamID);
    }
  };

  removeAllRemoteStreams = () => {
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

    // Send to relay
    this.socketConn.leaveVoiceChannel();

    this.removeAllRemoteStreams()
  };
}

const voiceManager = new VoiceManager();
export default voiceManager;
