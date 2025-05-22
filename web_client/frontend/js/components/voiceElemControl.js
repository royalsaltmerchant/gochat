import createElement from "./createElement.js";
import voiceManager from "../lib/voiceManager.js";
import voiceElemContainer from "./voiceElemContainer.js";

class VoiceElemControl {
  constructor() {
    this.domComponent = createElement("div", {
      class: "voice-elem-control",
    });

    this.audioElements = new Map();

    voiceManager.onStreamAdded = (stream) => {
      console.log("on stream added", stream)
      const voiceSub = voiceManager.voiceSubscriptions.find((sub) => {
        console.log("stream id and sub stream id", stream.id, sub.stream_id);
        return stream.id == sub.stream_id;
      });
      console.log("voice sub", voiceSub);

      const audio = document.createElement("audio");
      audio.className = "voice-elem";
      audio.id = `audio-item-${stream.id}`;
      audio.srcObject = stream;
      audio.autoplay = true;
      audio.controls = true;

      this.audioElements.set(stream.id, audio);

      // render
      if (voiceElemContainer.isOpen) {
        voiceElemContainer.render();
      }
    };

    voiceManager.onStreamRemoved = (streamID) => {
      const audio = this.audioElements.get(streamID);
      if (audio) {
        this.audioElements.delete(streamID);
      }

      // render
      if (voiceElemContainer.isOpen) {
        voiceElemContainer.render();
      }
    };
  }
}

const voiceElemControl = new VoiceElemControl();
export default voiceElemControl;
