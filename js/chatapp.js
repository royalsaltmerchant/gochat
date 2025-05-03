import createElement from "./components/createElement.js";
import isoDateFormat from "./lib/isoDateFormat.js";
import SocketConn from "./lib/socketConn.js";

class ChatApp {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.params = props.params;

    const pathParts = window.location.pathname.split('/');
    const channelUUID = pathParts[pathParts.length - 1];
    this.socketConn = new SocketConn({ channelUUID: channelUUID, chatapp: this });    

    this.chatBoxComponent = new ChatBoxComponent({
      domComponent: createElement("div"),
      socketConn: this.socketConn,
      channelUUID: channelUUID
    });

    this.render();
  }

  render = () => {
    this.domComponent.innerHTML = "";

    this.domComponent.append(this.chatBoxComponent.domComponent);
  };
}

class ChatBoxComponent {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.domComponent.className = "chat-box-container";
    this.socketConn = props.socketConn;
    this.channelUUID = props.channelUUID

    this.chatBoxMessagesComponent = new ChatBoxMessagesComponent({
      domComponent: createElement("div", {
        class: "chat-box-messages",
        id: "chat-box-messages",
      }),
      channelUUID: this.channelUUID
    });

    this.render();
  }

  render = () => {
    // clear
    this.domComponent.innerHTML = "";
    // render
    this.domComponent.append(
      this.chatBoxMessagesComponent.domComponent,
      createElement(
        "form",
        { class: "chat-box-form" },
        [
          createElement("input", {
            id: "chat-box-form-input",
            autofocus: true,
            type: "text",
            required: true,
            autocomplete: "off",
            placeholder: "Type message here...",
          }),
          createElement("button", { class: "chat-box-btn" }, "Send"),
        ],
        {
          type: "submit",
          event: (e) => {
            e.preventDefault();
            const content = e.target.elements["chat-box-form-input"].value;
            this.socketConn.sendMessage(content);

            // clear input
            e.target.elements["chat-box-form-input"].value = "";
            e.target.elements["chat-box-form-input"].focus();
          },
        }
      )
    );
  };
}

class ChatBoxMessagesComponent {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.chatBoxMessages = [];
    this.channelUUID = props.channelUUID

    this.init();
  }

  init = async () => {
    await this.getPreviousMessages()
    this.render()
  }

  getPreviousMessages = async () => {
    try {
      const response = await fetch("/api/get_messages", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({channelUUID: this.channelUUID}),
      });
      
      const result = await response.json();
      console.log(result);
      if (result.messages && result.messages.length) {
        this.chatBoxMessages = result.messages
      }
    } catch (error) {
      console.log(error);
    }
  }

  scrollDown = () => {
    this.domComponent.scrollTop = this.domComponent.scrollHeight;
  };

  renderMessages = () => {
    if (this.chatBoxMessages.length) {
      return [
        ...this.chatBoxMessages.map((data) =>
          createElement("div", { class: "chat-box-message-content" }, [
            createElement(
              "small",
              { style: "margin-right: var(--main-distance)" },
              isoDateFormat(data.Timestamp)
            ),
            createElement(
              "div",
              {
                style: "font-weight: bold; margin-right: var(--main-distance);",
              },
              `${data.Username}:`
            ),
            data.Content
          ])
        ),
      ];
    } else return []
  };

  render = () => {
    // clear
    this.domComponent.innerHTML = "";
    // render
    this.domComponent.append(...this.renderMessages());
  };
}

const chatApp = new ChatApp({
  domComponent: document.getElementById("chatapp"),
});
export default chatApp;
