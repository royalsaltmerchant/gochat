import createElement from "./createElement.js";
import mediaElemControl from "./mediaElemControl.js";
import voiceManager from "../lib/voiceManager.js";

class VideoContainer {
  constructor() {
    this.domComponent = createElement("div", { class: "video-container" });
    this.isOpen = false;

    // Register render callback
    mediaElemControl.onRender = () => {
      if (this.isOpen) {
        this.render();
      }
    };
  }

  open = () => {
    this.domComponent.style.display = "flex";
    this.isOpen = true;
    this.render();
    return this.domComponent;
  };

  close = () => {
    this.domComponent.style.display = "none";
    this.isOpen = false;
  };

  toggle = () => {
    if (this.isOpen) {
      this.close();
    } else {
      this.open();
    }
  };

  render = () => {
    this.domComponent.innerHTML = "";

    const header = createElement(
      "div",
      { class: "video-container-header" },
      [
        createElement("div", { class: "video-container-title" },
          voiceManager.isVideoEnabled() ? "Video Call" : "Voice Call"
        ),
        createElement(
          "button",
          { class: "video-container-close" },
          "×",
          { type: "click", event: () => this.close() }
        ),
      ]
    );

    const grid = createElement("div", { class: "video-grid" });

    // Add all media elements to grid
    const allElements = mediaElemControl.getAllElements();

    if (allElements.length === 0) {
      grid.appendChild(
        createElement("div", { class: "video-empty" }, "Waiting for participants...")
      );
    } else {
      // Determine grid layout based on participant count
      const count = allElements.length;
      let gridClass = "grid-1";
      if (count === 2) gridClass = "grid-2";
      else if (count <= 4) gridClass = "grid-4";
      else if (count <= 6) gridClass = "grid-6";
      else gridClass = "grid-many";

      grid.className = `video-grid ${gridClass}`;

      allElements.forEach((elem) => {
        grid.appendChild(elem);
      });
    }

    // Audio-only participants section (if any without video)
    const audioOnlyElements = mediaElemControl.getAudioOnlyElements();
    let audioSection = null;
    if (audioOnlyElements.length > 0 && voiceManager.isVideoEnabled()) {
      audioSection = createElement("div", { class: "audio-participants" }, [
        createElement("div", { class: "audio-participants-title" }, "Audio Only"),
        ...audioOnlyElements.map(elem => {
          const wrapper = createElement("div", { class: "audio-participant" });
          wrapper.appendChild(elem.cloneNode(true));
          return wrapper;
        })
      ]);
    }

    // Controls bar
    const controls = this.createControlBar();

    this.domComponent.appendChild(header);
    this.domComponent.appendChild(grid);
    if (audioSection) {
      this.domComponent.appendChild(audioSection);
    }
    this.domComponent.appendChild(controls);
  };

  createControlBar = () => {
    const controlBar = createElement("div", { class: "video-control-bar" });

    // Mute audio button
    const muteAudioBtn = createElement(
      "button",
      { class: "control-btn", id: "mute-audio-btn" },
      "🎤",
      {
        type: "click",
        event: () => {
          const localStream = voiceManager.getLocalStream();
          if (localStream) {
            const audioTrack = localStream.getAudioTracks()[0];
            if (audioTrack) {
              audioTrack.enabled = !audioTrack.enabled;
              muteAudioBtn.textContent = audioTrack.enabled ? "🎤" : "🎤‍🚫";
              muteAudioBtn.classList.toggle("muted", !audioTrack.enabled);
            }
          }
        },
      }
    );
    controlBar.appendChild(muteAudioBtn);

    // Mute video button (if video enabled)
    if (voiceManager.isVideoEnabled()) {
      const muteVideoBtn = createElement(
        "button",
        { class: "control-btn", id: "mute-video-btn" },
        "📹",
        {
          type: "click",
          event: () => {
            const localStream = voiceManager.getLocalStream();
            if (localStream) {
              const videoTrack = localStream.getVideoTracks()[0];
              if (videoTrack) {
                videoTrack.enabled = !videoTrack.enabled;
                muteVideoBtn.textContent = videoTrack.enabled ? "📹" : "📹‍🚫";
                muteVideoBtn.classList.toggle("muted", !videoTrack.enabled);
              }
            }
          },
        }
      );
      controlBar.appendChild(muteVideoBtn);
    }

    // Leave call button
    const leaveBtn = createElement(
      "button",
      { class: "control-btn leave-btn" },
      "📞",
      {
        type: "click",
        event: async () => {
          await voiceManager.leaveVoice();
          mediaElemControl.removeAllStreams();
          this.close();
        },
      }
    );
    controlBar.appendChild(leaveBtn);

    return controlBar;
  };
}

const videoContainer = new VideoContainer();
export default videoContainer;
