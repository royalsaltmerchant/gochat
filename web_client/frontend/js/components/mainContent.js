import { Component, createElement } from "../lib/foundation.js";
import ChatApp from "../chatapp.js";

export default class MainContentComponent extends Component {
  constructor(props) {
    super({
      domElem:
        props.domElem || createElement("div", { id: "channel-content", class: "channel-content" }),
      state: {
        currentChannelUUID: null,
      },
      autoInit: false,
      autoRender: props.autoRender,
    });
    this.data = props.data;
    this.socketConn = props.socketConn;

    this.chatApp = null;
    this.currentChannelUUID = null;

    this.onCleanup(() => {
      this.cleanup();
    });
  }

  getChatApp = () => {
    return this.useChild(
      "chat-app",
      () =>
        new ChatApp({
          domElem: createElement("div", { class: "chatapp-container" }),
          data: this.data,
          socketConn: this.socketConn,
        }),
      (child) => {
        child.data = this.data;
        child.socketConn = this.socketConn;
      }
    );
  };

  renderChannel = async (channelUUID) => {
    this.currentChannelUUID = channelUUID;
    await this.setState({ currentChannelUUID: channelUUID }, { render: false });

    const chatApp = this.getChatApp();
    this.chatApp = chatApp;
    this.chatApp.init?.(channelUUID);
    await this.render();
  };

  cleanup = () => {
    this.dropChild("chat-app");
    this.chatApp = null;
    this.currentChannelUUID = null;
    this.setState({ currentChannelUUID: null }, { render: false });
  };

  render = async () => {
    if (!this.state.currentChannelUUID) {
      if (this.chatApp) {
        this.cleanup();
      }

      return createElement("div", { class: "no-channel-selected" }, [
        createElement("h2", {}, "Select a channel to start chatting"),
      ]);
    }

    const chatApp = this.getChatApp();
    this.chatApp = chatApp;
    return chatApp.domElem;
  };
}
