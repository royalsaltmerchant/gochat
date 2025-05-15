import createElement from "./components/createElement.js";
import isoDateFormat from "./lib/isoDateFormat.js";
import { isImageUrl, createValidatedImage } from "./lib/imageValidation.js";

class ChatApp {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.socketConn = props.socketConn;
    this.chatBoxComponent = null;
  }

  initialize = (channelUUID) => {
    this.chatBoxComponent = new ChatBoxComponent({
      domComponent: createElement("div"),
      socketConn: this.socketConn,
      channelUUID: channelUUID,
    });

    this.render();
    // Get previous messages
    this.socketConn.getMessages();
  };

  render = () => {
    this.domComponent.innerHTML = "";
    if (this.chatBoxComponent) {
      this.domComponent.append(this.chatBoxComponent.domComponent);
    }
  };
}

class ChatBoxComponent {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.domComponent.className = "chat-box-container";
    this.socketConn = props.socketConn;
    this.channelUUID = props.channelUUID;

    this.chatBoxMessagesComponent = new ChatBoxMessagesComponent({
      domComponent: createElement("div", {
        class: "chat-box-messages",
        id: "chat-box-messages",
      }),
      channelUUID: this.channelUUID,
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
    this.channelUUID = props.channelUUID;
  }

  scrollDown = () => {
    this.domComponent.scrollTop = this.domComponent.scrollHeight;
  };

  isScrolledToBottom = () => {
    const offset = 40;
    return (
      this.domComponent.scrollHeight - this.domComponent.scrollTop <=
      this.domComponent.clientHeight + offset
    );
  };

  appendNewMessage = (data) => {
    this.chatBoxMessages.push(data);
    this.domComponent.append(this.createMessage(data, true));
  };

  createMessage = (data, isNew = false) => {
    console.log(data)
    // isNew is bool to render message with animation
    const parseMessageContent = (content) => {
      const urlRegexAll = /(https?:\/\/[^\s]+)/g;
      const urlRegex = /^https?:\/\/[^\s]+$/;
      const parts = content.split(urlRegexAll);

      return parts.map((part) => {
        if (urlRegex.test(part)) {
          if (isImageUrl(part)) {
            return createValidatedImage(part);
          } else {
            return createElement(
              "a",
              {
                href: part,
                target: "_blank",
                rel: "noopener noreferrer",
                style: "margin: 0 5px;",
              },
              part
            );
          }
        } else {
          return part;
        }
      });
    };

    const elem = createElement("div", { class: "chat-box-message-content" }, [
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
      ...parseMessageContent(data.content),
    ]);

    if (isNew) {
      elem.style.animation = "highlightFade 1s ease-out";
    }

    return elem;
  };

  renderMessages = () => {
    if (this.chatBoxMessages.length) {
      return this.chatBoxMessages.map((data) => this.createMessage(data));
    } else {
      return [];
    }
  };

  render = () => {
    // clear
    this.domComponent.innerHTML = "";
    // render
    this.domComponent.append(...this.renderMessages());
  };
}

// Export the class instead of an instance
export default ChatApp;
