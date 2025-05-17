import createElement from "./createElement.js";
import ChatApp from "../chatapp.js"

export default class MainContentComponent {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.chatApp = null;
    this.currentChannelUUID = null;
    this.render();
  }

  renderChannel = (channelUUID) => {
    this.currentChannelUUID = channelUUID;
    this.domComponent.innerHTML = "";
    const chatAppDiv = createElement("div", { class: "chatapp-container" });
    this.domComponent.append(chatAppDiv);
    if (!this.chatApp) {
      this.chatApp = new ChatApp({ domComponent: chatAppDiv, data: this.data, socketConn: this.socketConn });
    } else {
      this.chatApp.domComponent = chatAppDiv;
    }
    this.chatApp.initialize(channelUUID);
  };

  cleanup = () => {
    if (this.chatApp) {
      // Clean up any WebSocket connections or event listeners
      this.chatApp.cleanup?.();
      this.chatApp = null;
    }
    this.currentChannelUUID = null;
  };

  render = () => {
    this.cleanup();
    this.domComponent.innerHTML = "";
    this.domComponent.append(
      createElement("div", { class: "no-channel-selected" }, [
        createElement("h2", {}, "Select a channel to start chatting"),
      ])
    );
  };
}