import createElement from "./components/createElement.js";
import isoDateFormat from "./lib/isoDateFormat.js";
import { isImageUrl, createValidatedImage } from "./lib/imageValidation.js";
import voiceManager from "./lib/voiceManager.js";
import voiceElemContainer from "./components/voiceElemContainer.js";

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
      channelUUID: channelUUID,
    });

    this.render();
    // Init with first batch of previous messages
    const anchorTime = new Date().toISOString(); // RFC3339 format
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

    this.voiceChatComponent = new VoiceChatComponent({
      domComponent: createElement("div", {
        class: "voice-chat-component",
        id: "voice-chat-component",
      }),
      channelUUID: this.channelUUID,
      user: this.data.user,
    });

    this.render();
  }

  renderVoiceChatComponent = () => {
    if (this.channel.allow_voice == 1) {
      return this.voiceChatComponent.domComponent;
    } else return "";
  };

  render = () => {
    // clear
    this.domComponent.innerHTML = "";
    // render
    this.domComponent.append(
      this.chatBoxMessagesComponent.domComponent,
      this.renderVoiceChatComponent(),
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
                  e.preventDefault(); // prevent newline
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

    // Virtual scroll state
    this.visibleStartIndex = 0;
    this.visibleEndIndex = 50;
    this.messageHeightEstimate = 100;
    this.renderThreshold = 10;
    this.lastRenderStartIndex = -1;

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
          this.onScrollThrottled();
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

    if (wasAtBottom) {
      this.visibleStartIndex = Math.max(
        0,
        this.chatBoxMessages.length - this.messageRequestSize
      );
      this.visibleEndIndex = this.chatBoxMessages.length;
      this.lastRenderStartIndex = this.visibleStartIndex;
      this.renderVirtual();
      this.scrollDown();
    }
  };

  createMessage = (data, isNew = false) => {
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
      createElement("div", { style: "display: flex; align-items: flex-end;" }, [
        createElement(
          "small",
          {
            style:
              "margin-right: var(--main-distance); color: var(--main-gray);",
          },
          isoDateFormat(data.timestamp)
        ),
        createElement(
          "div",
          {
            style:
              "font-weight: bold; margin-right: var(--main-distance); color: var(--light-yellow);",
          },
          data.username
        ),
      ]),
      createElement("div", { class: "chat-box-message-text" }, [
        ...parseMessageContent(data.content),
      ]),
      createElement("hr"),
    ]);

    if (isNew) {
      elem.style.animation = "highlightFade 1s ease-out";
    }

    return elem;
  };

  onScrollThrottled = () => {
    if (this.scrollTimeout) clearTimeout(this.scrollTimeout);
    this.scrollTimeout = setTimeout(() => this.updateVisibleWindow(), 100);
  };

  updateVisibleWindow = () => {
    const scrollTop = this.domComponent.scrollTop;
    const containerHeight = this.domComponent.clientHeight;
    const buffer = 10;

    const startIndex = Math.max(
      0,
      Math.floor(scrollTop / this.messageHeightEstimate) - buffer
    );
    const endIndex = Math.min(
      this.chatBoxMessages.length,
      Math.ceil((scrollTop + containerHeight) / this.messageHeightEstimate) +
        buffer
    );

    const shouldRender =
      Math.abs(startIndex - this.lastRenderStartIndex) >= this.renderThreshold;

    if (!shouldRender) {
      return;
    }

    this.visibleStartIndex = startIndex;
    this.visibleEndIndex = endIndex;
    this.lastRenderStartIndex = startIndex;
    this.renderVirtual();
  };

  renderVirtual = () => {
    const topHeight = this.visibleStartIndex * this.messageHeightEstimate;
    const bottomHeight =
      (this.chatBoxMessages.length - this.visibleEndIndex) *
      this.messageHeightEstimate;

    const topSpacer = createElement("div", {
      style: `height: ${topHeight}px;`,
    });

    const bottomSpacer = createElement("div", {
      style: `height: ${bottomHeight}px;`,
    });

    const visibleMessages = this.chatBoxMessages
      .slice(this.visibleStartIndex, this.visibleEndIndex)
      .map((data) => this.createMessage(data));

    this.domComponent.innerHTML = "";
    this.domComponent.append(topSpacer, ...visibleMessages, bottomSpacer);
  };

  renderMessages = () => {
    return this.chatBoxMessages.map((data) => this.createMessage(data));
  };

  render = () => {
    this.renderVirtual();
  };
}

class VoiceChatComponent {
  constructor(props) {
    this.domComponent = props.domComponent;
    this.channelUUID = props.channelUUID;
    this.user = props.user;

    this.voiceManager = null;

    this.render();
  }

  renderVoiceComponent = () => {
    if (voiceManager.currentChannelUUID === this.channelUUID) {
      return createElement("div", { class: "voice-chat-toolbar" }, [
        createElement(
          "button",
          { class: "voice-leave-btn btn-red", style: "margin-right: 5px;" },
          "🔊 Leave Voice",
          {
            type: "click",
            event: async () => {
              await voiceManager.leaveVoice();
              if (voiceElemContainer.isOpen) {
                voiceElemContainer.close();
              }
              this.render();
            },
          }
        ),
        createElement("button", { class: "voice-options-btn" }, "Show Voices", {
          type: "click",
          event: () => {
            if(voiceElemContainer.isOpen) return;
            voiceElemContainer.open();
          },
        }),
      ]);
    } else {
      return createElement(
        "button",
        {
          class: "chat-box-btn",
        },
        "🔊 Join Voice",
        {
          type: "click",
          event: async () => {
            await voiceManager.leaveVoice();
            await voiceManager.joinVoice({
              channelUUID: this.channelUUID,
              userID: this.user.id,
            });

            this.render();
            voiceElemContainer.open();
          },
        }
      );
    }
  };

  render = () => {
    this.domComponent.innerHTML = "";
    this.domComponent.append(this.renderVoiceComponent());
  };
}

// Export the class instead of an instance
export default ChatApp;
