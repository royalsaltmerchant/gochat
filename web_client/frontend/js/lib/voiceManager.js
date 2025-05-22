import RTCConn from "./rtcConn.js";

class VoiceManager {
  constructor() {
    this.currentRTCConn = null;
    this.currentChannelUUID = null;
    this.audioCtx = null;
  }

  initAudio = async () => {
    console.log("initializing audio");
    this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    await this.audioCtx.resume(); // <- this unlocks autoplay
    console.log("audio context", this.audioCtx);
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
    });

    console.log("current RTC conn", this.currentRTCConn);

    await this.currentRTCConn.init();
    await this.currentRTCConn.start(); // Connects to SFU and joins room
  };

  leaveVoice = async () => {
    console.log("leaving voice from voice manager");
    this.currentChannelUUID = null;

    if (this.currentRTCConn) {
      this.currentRTCConn.close();
      this.currentRTCConn = null;
    }

    if (this.audioCtx) {
      await this.audioCtx.close();
      this.audioCtx = null;
    }
  };
}

const voiceManager = new VoiceManager();
export default voiceManager;
