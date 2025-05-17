import RTCConn from "./rtcConn.js";

export default class VoiceManager {
  constructor() {
    this.currentRTCConn = null;
  }

  joinVoice({ room, userID }) {
    if (this.currentRTCConn) {
      this.currentRTCConn.close();
    }

    const peer1 = new RTCConn({
      room,
      userID: "first",
    });
    
    const peer2 = new RTCConn({
      room,
      userID: "second",
    });
    peer1.start()
    peer2.start()
    // this.currentRTCConn.start(); // Connects to SFU and joins room
  }

  leaveVoice() {
    if (this.currentRTCConn) {
      this.currentRTCConn.close();
      this.currentRTCConn = null;
    }
  }
}
