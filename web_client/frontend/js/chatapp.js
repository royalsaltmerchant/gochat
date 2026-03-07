import createElement from "./components/createElement.js";
import isoDateFormat from "./lib/isoDateFormat.js";
import { isImageUrl, createValidatedImage } from "./lib/imageValidation.js";
import identityManager from "./lib/identityManager.js";
import e2ee from "./lib/e2ee.js";
import platform from "./platform/index.js";

class ChatApp {
  constructor(props) {
    this.domElem = props.domElem || createElement("div");
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.chatBoxComponent = null;
  }

  init = (channelUUID) => {
    this.chatBoxComponent?.destroy?.();

    this.chatBoxComponent = new ChatBoxComponent({
      domElem: createElement("div"),
      data: this.data,
      socketConn: this.socketConn,
      channelUUID,
    });

    this.render();
    const anchorTime = new Date().toISOString();
    this.socketConn.getMessages(anchorTime, this.chatBoxComponent?.space?.uuid);
  };

  destroy = () => {
    this.chatBoxComponent?.destroy?.();
    this.chatBoxComponent = null;
    this.domElem.innerHTML = "";
  };

  render = () => {
    this.domElem.innerHTML = "";
    if (this.chatBoxComponent) {
      this.domElem.append(this.chatBoxComponent.domElem);
    }
  };
}

class ChatBoxComponent {
  constructor(props) {
    this.domElem = props.domElem || createElement("div");
    this.domElem.className = "chat-box-container";
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
      domElem: createElement("div", {
        class: "chat-box-messages",
        id: "chat-box-messages",
      }),
      channelUUID: this.channelUUID,
      spaceUUID: this.space?.uuid || "",
      socketConn: this.socketConn,
    });

    this.render();
  }

  render = () => {
    this.domElem.innerHTML = "";
    this.domElem.append(
      createElement("div", { class: "chatapp-channel-title" }, this.channel.name),
      this.chatBoxMessagesComponent.domElem,
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
              event: async (e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  const textarea = e.target;
                  const content = textarea.value.trim();
                  if (content) {
                    try {
                      const identity = await identityManager.getOrCreateIdentity();
                      const recipients = (this.space?.users || [])
                        .filter(
                          (user) =>
                            user?.public_key &&
                            user?.enc_public_key
                        )
                        .map((user) => ({
                          authPublicKey: user.public_key,
                          encPublicKey: user.enc_public_key,
                        }));

                      const envelope = await e2ee.encryptMessageForSpace({
                        plaintext: content,
                        spaceUUID: this.space?.uuid,
                        channelUUID: this.channelUUID,
                        identity,
                        recipients,
                      });

                      this.socketConn.sendMessage({ envelope }, this.space?.uuid);
                      textarea.value = "";
                      textarea.style.height = "auto";
                      textarea.focus();
                    } catch (err) {
                      console.error(err);
                      platform.alert(
                        err?.message || "Failed to encrypt message"
                      );
                    }
                  }
                }
              },
            },
          ]
        ),
      ])
    );
  };

  destroy = () => {
    this.chatBoxMessagesComponent?.destroy?.();
    this.chatBoxMessagesComponent = null;
    this.domElem.innerHTML = "";
  };
}

class ChatBoxMessagesComponent {
  constructor(props) {
    this.chatBoxMessages = [];
    this.channelUUID = props.channelUUID;
    this.spaceUUID = props.spaceUUID || "";
    this.socketConn = props.socketConn;

    this.messageRequestSize = 50;
    this.hasMoreMessages = true;
    this.debounceTimeout = null;
    this.isLoading = false;

    this.domElem = createElement(
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
    this.socketConn.getMessages(oldestTimestamp, this.spaceUUID);
  };

  scrollDown = () => {
    this.domElem.scrollTop = this.domElem.scrollHeight;
  };

  isScrolledToTop = () => {
    return this.domElem.scrollTop <= 20;
  };

  isScrolledToBottom = () => {
    const offset = 40;
    return (
      this.domElem.scrollHeight - this.domElem.scrollTop <=
      this.domElem.clientHeight + offset
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
    this.domElem.innerHTML = "";
    const allMessages = this.chatBoxMessages.map((data) => this.createMessage(data));
    this.domElem.append(...allMessages);
  };

  destroy = () => {
    if (this.debounceTimeout) {
      clearTimeout(this.debounceTimeout);
      this.debounceTimeout = null;
    }
    this.domElem.innerHTML = "";
  };
}

export default ChatApp;
