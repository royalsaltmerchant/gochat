import RTCConn from "./rtcConn.js";

export default class VoiceManager {
  constructor(audioCtx) {
    this.currentRTCConn = null;
    this.audioCtx = audioCtx;
  }

  joinVoice({ room, userID }) {
    if (this.currentRTCConn) {
      this.currentRTCConn.close();
    }

    this.currentRTCConn = new RTCConn({
      audioCtx: this.audioCtx,
      room,
      userID: userID,
    });
    
    // const peer2 = new RTCConn({
    //   room,
    //   userID: "second",
    // });
    
    // peer1.start()
    // peer2.start()
    this.currentRTCConn.start(); // Connects to SFU and joins room
  }

  leaveVoice() {
    if (this.currentRTCConn) {
      this.currentRTCConn.close();
      this.currentRTCConn = null;
    }
  }
}
