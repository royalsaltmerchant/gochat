import createElement from "./createElement.js";

export default class DashModal {
  constructor(app) {
    this.app = app;
    this.domComponent = createElement("div", { class: "modal" });
    this.currentSpace = null;
    this.render();
  }

  open = (space) => {
    this.currentSpace = space;
    this.domComponent.style.display = "block";
    this.render();
  };

  close = () => {
    this.domComponent.style.display = "none";
    this.currentSpace = null;
  };

  createNewChannel = async () => {
    if (!this.currentSpace) return;
    const channelName = window.prompt("Please enter Channel 'Name'");
    if (!channelName) return;
    try {
      const response = await fetch("/api/new_channel", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: channelName,
          spaceUUID: this.currentSpace.UUID,
        }),
      });
      const result = await response.json();
      if (result.channel) {
        this.currentSpace.Channels.push(result.channel);
        // Update the space component directly instead of re-rendering the whole sidebar
        const spaceComponent = this.app.sidebar.spaceComponents.find(
          (comp) => comp.space.UUID === this.currentSpace.UUID
        );
        if (spaceComponent) {
          spaceComponent.render();
        }
        this.render();
      }
    } catch (error) {
      console.log(error);
    }
  };

  deleteChannel = async (channelUUID) => {
    if (!this.currentSpace) return;
    if (
      !window.confirm(
        "Are you sure you want to delete this channel? This action cannot be undone."
      )
    )
      return;
    try {
      const response = await fetch(`/api/channel/${channelUUID}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (response.ok) {
        // Update the channels array - this will automatically update the app's data structure
        this.currentSpace.Channels = this.currentSpace.Channels.filter(
          (c) => c.UUID !== channelUUID
        );

        // Check if the current channel still exists in any space
        const channelStillExists = this.app.data.spaces.some((space) =>
          space.Channels.some(
            (channel) =>
              channel.UUID === this.app.mainContent.currentChannelUUID
          )
        );

        if (!channelStillExists) {
          this.app.mainContent.render();
        }

        // Update the space component directly instead of re-rendering the whole sidebar
        const spaceComponent = this.app.sidebar.spaceComponents.find(
          (comp) => comp.space.UUID === this.currentSpace.UUID
        );
        if (spaceComponent) {
          spaceComponent.render();
        }
        this.render();
      } else {
        const result = await response.json();
        window.alert(`ERROR: ${result.error}`);
      }
    } catch (error) {
      console.log(error);
    }
  };

  renderAuthorSettings = () => {
    return [
      // Space Management Section
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Space Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn" }, "Invite User", {
            type: "click",
            event: this.app.inviteUser,
          }),
          createElement("button", { class: "btn btn-red" }, "Delete Space", {
            type: "click",
            event: this.app.deleteSpace,
          }),
        ]),
      ]),
      // Channel Management Section
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Channel Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn" }, "+ Create Channel", {
            type: "click",
            event: this.createNewChannel,
          }),
        ]),
        createElement(
          "div",
          { class: "channels-list" },
          this.currentSpace.Channels.map((channel) =>
            createElement("div", { class: "channel-item" }, [
              createElement("span", {}, channel.Name),
              createElement(
                "button",
                { class: "btn-small btn-red" },
                "Delete",
                {
                  type: "click",
                  event: () => this.deleteChannel(channel.UUID),
                }
              ),
            ])
          )
        ),
      ]),
    ];
  };

  renderSpaceUserSettings = () => {
    return [
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Space Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn btn-red" }, "Leave Space", {
            type: "click",
            event: async () => {
              try {
                const response = await fetch(
                  `/api/space_user_self/${this.currentSpace.UUID}`,
                  {
                    method: "DELETE",
                    credentials: "include",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                      spaceUUID: this.currentSpace.UUID,
                    }),
                  }
                );
                if (response.ok) {
                  // Handled by socket remove-user with dashboard handleRemoveUser
                  this.close();
                } else {
                  const result = await response.json();
                  window.alert(`ERROR: ${result.error}`);
                }
              } catch (error) {
                console.log(error);
              }
            },
          }),
        ]),
      ]),
    ];
  };

  render = () => {
    if (!this.currentSpace) {
      this.close();
      return;
    }

    this.domComponent.innerHTML = "";

    const isAuthor = this.currentSpace.AuthorID === this.app.data.user.ID;

    this.domComponent.append(
      createElement("div", { class: "modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, `Space Settings: ${this.currentSpace.Name}`),
          createElement("button", { class: "close-modal" }, "×", {
            type: "click",
            event: this.close,
          }),
        ]),
        createElement(
          "div",
          { class: "modal-body" },
          isAuthor
            ? this.renderAuthorSettings()
            : this.renderSpaceUserSettings()
        ),
      ])
    );
  };
}
