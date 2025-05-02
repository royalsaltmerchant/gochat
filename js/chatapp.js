import createElement from "./components/createElement.js";
import isoDateFormat from "./lib/isoDateFormat.js";
import SocketConn from "./lib/socketConn.js";

class ChatApp {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.params = props.params;

    const pathParts = window.location.pathname.split('/');
    const roomUUID = pathParts[pathParts.length - 1];
    this.socketConn = new SocketConn({ roomUUID: roomUUID, chatapp: this });    

    this.chatBoxComponent = new ChatBoxComponent({
      domComponent: createElement("div"),
      socketConn: this.socketConn,
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

    this.chatBoxMessagesComponent = new ChatBoxMessagesComponent({
      domComponent: createElement("div", {
        class: "chat-box-messages",
        id: "chat-box-messages",
      }),
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

    this.render();
  }

  scrollDown = () => {
    this.domComponent.scrollTop = this.domComponent.scrollHeight;
  };

  renderMessages = () => {
    return [
      ...this.chatBoxMessages.map((data) =>
        createElement("div", { class: "chat-box-message-content" }, [
          createElement(
            "small",
            { style: "margin-right: var(--main-distance)" },
            isoDateFormat(data.timestamp)
          ),
          createElement(
            "div",
            {
              style: "font-weight: bold; margin-right: var(--main-distance);",
            },
            `${data.username}:`
          ),
          data.message
        ])
      ),
    ];
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
