import createElement from "./createElement.js";
import voiceManager from "../lib/voiceManager.js";

class MediaElemControl {
  constructor() {
    this.domComponent = createElement("div", {
      class: "media-elem-control",
    });

    this.mediaElements = new Map(); // streamId -> { stream, container, hasVideo }
    this.localStreamElement = null;

    console.log("📺 MediaElemControl initialized, setting callbacks");

    // Store reference to old callbacks if they exist
    const oldOnStreamAdded = voiceManager.onStreamAdded;
    const oldOnStreamRemoved = voiceManager.onStreamRemoved;

    voiceManager.onStreamAdded = (stream) => {
      console.log("📺 onStreamAdded called", stream.id);
      this.addStream(stream, false);
      // Also call old handler for backwards compatibility (voiceElemControl)
      if (oldOnStreamAdded) oldOnStreamAdded(stream);
    };

    voiceManager.onStreamRemoved = (streamID) => {
      console.log("📺 onStreamRemoved called", streamID);
      this.removeStream(streamID);
      if (oldOnStreamRemoved) oldOnStreamRemoved(streamID);
    };

    voiceManager.onLocalStream = (stream) => {
      console.log("📺 onLocalStream called", stream.id, "video tracks:", stream.getVideoTracks().length);
      this.setLocalStream(stream);
    };
  }

  addStream = (stream, isLocal = false) => {
    if (this.mediaElements.has(stream.id)) return;

    const voiceSub = voiceManager.voiceSubscriptions.find((sub) => {
      return stream.id == sub.stream_id;
    });

    const hasVideo = stream.getVideoTracks().length > 0;
    const hasAudio = stream.getAudioTracks().length > 0;

    const container = document.createElement("div");
    container.className = `media-container ${hasVideo ? "has-video" : "audio-only"} ${isLocal ? "local" : "remote"}`;
    container.dataset.streamId = stream.id;

    // Username label
    const usernameLabel = document.createElement("div");
    usernameLabel.className = "media-username";
    usernameLabel.textContent = isLocal ? "You" : (voiceSub?.username || "Unknown");
    container.appendChild(usernameLabel);

    // Video element (if video track exists)
    if (hasVideo) {
      const video = document.createElement("video");
      video.className = "media-video";
      video.srcObject = stream;
      video.autoplay = true;
      video.playsInline = true;
      video.muted = isLocal; // Mute local video to prevent echo
      container.appendChild(video);
    }

    // Audio element (always create for remote streams)
    if (hasAudio && !isLocal) {
      const audio = document.createElement("audio");
      audio.className = "media-audio";
      audio.srcObject = stream;
      audio.autoplay = true;
      container.appendChild(audio);
    }

    // Controls
    const controls = this.createControls(stream, isLocal, hasVideo, hasAudio);
    container.appendChild(controls);

    // Track removal
    stream.onremovetrack = (event) => {
      if (stream.getTracks().length === 0) {
        this.removeStream(stream.id);
      }
    };

    this.mediaElements.set(stream.id, {
      stream,
      container,
      hasVideo,
      hasAudio,
      isLocal,
      username: voiceSub?.username || (isLocal ? "You" : "Unknown"),
    });

    this.render();
  };

  setLocalStream = (stream) => {
    this.addStream(stream, true);
  };

  createControls = (stream, isLocal, hasVideo, hasAudio) => {
    const controls = document.createElement("div");
    controls.className = "media-controls";

    if (hasAudio && !isLocal) {
      // Volume control for remote audio
      const volumeBtn = document.createElement("button");
      volumeBtn.className = "media-btn";
      volumeBtn.textContent = "🔊";
      volumeBtn.onclick = () => {
        const audio = controls.parentElement.querySelector("audio");
        if (audio) {
          audio.muted = !audio.muted;
          volumeBtn.textContent = audio.muted ? "🔇" : "🔊";
        }
      };
      controls.appendChild(volumeBtn);
    }

    if (hasVideo) {
      // Fullscreen button
      const fullscreenBtn = document.createElement("button");
      fullscreenBtn.className = "media-btn";
      fullscreenBtn.textContent = "⛶";
      fullscreenBtn.onclick = () => {
        const video = controls.parentElement.querySelector("video");
        if (video) {
          if (video.requestFullscreen) {
            video.requestFullscreen();
          } else if (video.webkitRequestFullscreen) {
            video.webkitRequestFullscreen();
          }
        }
      };
      controls.appendChild(fullscreenBtn);
    }

    return controls;
  };

  removeStream = (streamID) => {
    const entry = this.mediaElements.get(streamID);
    if (!entry) return;

    this.mediaElements.delete(streamID);
    this.render();
  };

  removeAllStreams = () => {
    this.mediaElements.clear();
    this.render();
  };

  getVideoElements = () => {
    return Array.from(this.mediaElements.values())
      .filter((v) => v.hasVideo)
      .map((v) => v.container);
  };

  getAudioOnlyElements = () => {
    return Array.from(this.mediaElements.values())
      .filter((v) => !v.hasVideo)
      .map((v) => v.container);
  };

  getAllElements = () => {
    return Array.from(this.mediaElements.values()).map((v) => v.container);
  };

  render = () => {
    console.log("📺 mediaElemControl.render() called, elements:", this.mediaElements.size);
    // This will be called by the container to update display
    if (this.onRender) {
      this.onRender();
    }
  };
}

const mediaElemControl = new MediaElemControl();
export default mediaElemControl;
