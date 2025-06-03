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
      const voiceSub = voiceManager.voiceSubscriptions.find((sub) => {
        return stream.id == sub.stream_id;
      });

      const container = document.createElement("div");
      container.className = "audio-container";

      const audio = document.createElement("audio");
      audio.className = "voice-elem";
      audio.id = `audio-item-${stream.id}`;
      audio.srcObject = stream;
      audio.autoplay = true;
      audio.controls = false; // We're making custom controls

      const playPauseBtn = document.createElement("div");
      playPauseBtn.style.cursor = "pointer";
      playPauseBtn.textContent = "🔊";
      playPauseBtn.onclick = () => {
        if (audio.paused) {
          audio.play();
          playPauseBtn.textContent = "🔊";
        } else {
          audio.pause();
          playPauseBtn.textContent = "🔇";
        }
      };

      const volumeSlider = document.createElement("input");
      volumeSlider.type = "range";
      volumeSlider.min = "0";
      volumeSlider.max = "1";
      volumeSlider.step = "0.01";
      volumeSlider.value = audio.volume;
      volumeSlider.oninput = (e) => {
        audio.volume = e.target.value;
      };

      container.appendChild(audio);
      container.appendChild(playPauseBtn);
      container.appendChild(volumeSlider);

      if (voiceSub) {
        container.username = voiceSub.username;
      }

      this.audioElements.set(stream.id, container);

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
