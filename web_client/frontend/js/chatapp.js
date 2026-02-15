import createElement from "./components/createElement.js";
import isoDateFormat from "./lib/isoDateFormat.js";
import { isImageUrl, createValidatedImage } from "./lib/imageValidation.js";

class ChatApp {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.chatBoxComponent = null;
  }

  initialize = (channelUUID) => {
    this.chatBoxComponent = new ChatBoxComponent({
      domComponent: createElement("div"),
      data: this.data,
      socketConn: this.socketConn,
      channelUUID,
    });

    this.render();
    const anchorTime = new Date().toISOString();
    this.socketConn.getMessages(anchorTime);
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
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.channelUUID = props.channelUUID;

    for (const space of this.data.spaces) {
      const channel = space.channels.find((c) => c.uuid === this.channelUUID);
      if (channel) {
        this.space = space;
        this.channel = channel;
        break;
      }
    }

    this.chatBoxMessagesComponent = new ChatBoxMessagesComponent({
      domComponent: createElement("div", {
        class: "chat-box-messages",
        id: "chat-box-messages",
      }),
      channelUUID: this.channelUUID,
      socketConn: this.socketConn,
    });

    this.render();
  }

  render = () => {
    this.domComponent.innerHTML = "";
    this.domComponent.append(
      createElement("div", { class: "chatapp-channel-title" }, this.channel.name),
      this.chatBoxMessagesComponent.domComponent,
      createElement("div", { class: "chat-box-form" }, [
        createElement(
          "textarea",
          {
            id: "chat-box-form-input",
            class: "chat-box-form-input",
            autofocus: true,
            type: "text",
            required: true,
            autocomplete: "off",
            placeholder: "Type message here...",
          },
          null,
          [
            {
              type: "input",
              event: (e) => {
                const textarea = e.target;
                textarea.style.height = "auto";
                textarea.style.height = `${textarea.scrollHeight}px`;
              },
            },
            {
              type: "keydown",
              event: (e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  const textarea = e.target;
                  const content = textarea.value.trim();
                  if (content) {
                    this.socketConn.sendMessage(content);
                    textarea.value = "";
                    textarea.style.height = "auto";
                    textarea.focus();
                  }
                }
              },
            },
          ]
        ),
      ])
    );
  };
}

class ChatBoxMessagesComponent {
  constructor(props) {
    this.chatBoxMessages = [];
    this.channelUUID = props.channelUUID;
    this.socketConn = props.socketConn;

    this.messageRequestSize = 50;
    this.hasMoreMessages = true;
    this.debounceTimeout = null;
    this.isLoading = false;

    this.domComponent = createElement(
      "div",
      {
        class: "chat-box-messages",
        id: "chat-box-messages",
      },
      null,
      {
        type: "scroll",
        event: () => {
          this.getPreviousMessagesDebounce();
        },
      }
    );
  }

  getPreviousMessagesDebounce = () => {
    if (this.debounceTimeout) clearTimeout(this.debounceTimeout);
    this.debounceTimeout = setTimeout(() => {
      this.getPreviousMessages();
    }, 300);
  };

  getPreviousMessages = () => {
    if (
      this.isLoading ||
      !this.hasMoreMessages ||
      !this.chatBoxMessages.length ||
      !this.isScrolledToTop()
    )
      return;

    this.isLoading = true;
    const oldestTimestamp = this.chatBoxMessages[0].timestamp;
    this.socketConn.getMessages(oldestTimestamp);
  };

  scrollDown = () => {
    this.domComponent.scrollTop = this.domComponent.scrollHeight;
  };

  isScrolledToTop = () => {
    return this.domComponent.scrollTop <= 20;
  };

  isScrolledToBottom = () => {
    const offset = 40;
    return (
      this.domComponent.scrollHeight - this.domComponent.scrollTop <=
      this.domComponent.clientHeight + offset
    );
  };

  appendNewMessage = (data) => {
    const wasAtBottom = this.isScrolledToBottom();
    this.chatBoxMessages.push(data);
    this.render();
    if (wasAtBottom) {
      requestAnimationFrame(() => this.scrollDown());
    }
  };

  createMessage = (data) => {
    const parseMessageContent = (content) => {
      const urlRegexAll = /(https?:\/\/[^\s]+)/g;
      const urlRegex = /^https?:\/\/[^\s]+$/;
      const parts = content.split(urlRegexAll);

      return parts.map((part) => {
        if (urlRegex.test(part)) {
          if (isImageUrl(part)) {
            return createValidatedImage(part);
          }
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
        return part;
      });
    };

    return createElement("div", { class: "chat-box-message-content" }, [
      createElement("div", { style: "display: flex; align-items: flex-end;" }, [
        createElement(
          "small",
          {
            style: "margin-right: var(--main-distance); color: var(--main-gray);",
          },
          isoDateFormat(data.timestamp)
        ),
        createElement(
          "div",
          {
            style: "font-weight: bold; margin-right: var(--main-distance); color: var(--light-yellow);",
          },
          data.username
        ),
      ]),
      createElement("div", { class: "chat-box-message-text" }, [
        ...parseMessageContent(data.content),
      ]),
      createElement("hr"),
    ]);
  };

  render = () => {
    this.domComponent.innerHTML = "";
    const allMessages = this.chatBoxMessages.map((data) => this.createMessage(data));
    this.domComponent.append(...allMessages);
  };
}

export default ChatApp;
