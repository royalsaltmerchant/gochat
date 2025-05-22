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
        createElement(
          "div",
          {
            style:
              "display: flex; align-items: baseline; justify-content: space-between;",
          },
          [
            createElement("div", {}, "Voices"),
            createElement("div", { style: "cursor: pointer; font-size: 30px;" }, "×", {
              type: "click",
              event: () => {
                this.close();
              },
            }),
          ]
        ),
        ...Array.from(voiceElemControl.audioElements.values(), (audioElem) =>
          createElement("div", { class: "voice-elem-wrapper" }, [
            createElement(
              "div",
              { style: "margin-right: 5px;" },
              audioElem.username ? audioElem.username : "Unknown"
            ),
            audioElem,
          ])
        ),
      ])
    );
  };
}

const voiceElemContainer = new VoiceElemContainer();
export default voiceElemContainer;
