import createElement from "./createElement.js";
import voiceElemControl from "./voiceElemControl.js";

class VoiceElemContainer {
  constructor() {
    this.domComponent = createElement("div", { class: "voice-elem-container" });
    this.isOpen = false;
  }

  open = (props) => {
    this.domComponent.style.display = "block";
    this.isOpen = true;
    return this.render(props);
  };

  close = () => {
    this.domComponent.style.display = "none";
    this.isOpen = false;
  };

  render = () => {
    this.domComponent.innerHTML = "";
    this.domComponent.append(
      createElement("div", { class: "voice-elem-container-content" }, [
        createElement("div", {}, "Voices"),
        ...Array.from(voiceElemControl.audioElements.values(), (audio) =>
          createElement("div", { class: "voice-elem-wrapper" }, [
            createElement("div", {}, "Username"),
            audio,
          ])
        ),
      ])
    );
  };
}

const voiceElemContainer = new VoiceElemContainer();
export default voiceElemContainer;
