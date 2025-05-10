import createElement from "./createElement.js";

export default class SidebarComponent {
  constructor(params) {
    this.data = params.data;
    this.getCurrentSpaceUUID = params.getCurrentSpaceUUID;
    this.spaceComponents = [];
    this.domComponent = createElement("div", { class: "sidebar" });
    this.createNewSpace = params.createNewSpace;
    this.openSpaceSettings = params.openSpaceSettings;
    this.loadChannel = params.loadChannel;

    this.render();
  }

  initSpaceComponents = () => {
    if (this.data.spaces) {
      this.spaceComponents = this.data.spaces.map(
        (space) =>
          new SpaceItemComponent(
            space,
            this.data.user,
            this.openSpaceSettings,
            this.loadChannel
          )
      );
    }
  };

  addSpace = (space) => {
    const comp = new SpaceItemComponent(
      space,
      this.data.user,
      this.openSpaceSettings,
      this.loadChannel
    );
    this.spaceComponents.push(comp);
    this.render();
  };

  render = () => {
    this.domComponent.innerHTML = "";

    this.initSpaceComponents();

    this.spaceUserListComponent = new SpaceUserListComponent(
      this.data,
      this.getCurrentSpaceUUID
    );

    // Spaces
    this.domComponent.append(
      createElement("div", { class: "sidebar-spaces-container" }, [
        createElement(
          "div",
          { id: "spaces-list" },
          this.spaceComponents.map((comp) => comp.domComponent)
        ),
        createElement(
          "button",
          { class: "create-space-btn", id: "new-space-btn" },
          "+ Create New Space",
          { type: "click", event: this.createNewSpace }
        ),
      ]),
      createElement("br"),
      this.spaceUserListComponent.domComponent
    );
  };
}

class SpaceUserListComponent {
  constructor(data, getCurrentSpaceUUID) {
    this.data = data;
    this.getCurrentSpaceUUID = getCurrentSpaceUUID;
    this.domComponent = createElement("div", {
      class: "space-users-list",
      id: "space-users-list",
    });

    this.render();
  }

  isAuthor = (space, user) => {
    return String(space.AuthorID) === String(user.ID); // the current user not space user
  };

  renderUserElement = (currentSpace, user) => {
    return createElement(
      "div",
      { class: "space-user-item", id: "space-user-item" },
      `${user.Username} ${this.isAuthor(currentSpace, user) ? "*" : ""}`,
      {
        type: "click",
        event: async () => {
          if (this.isAuthor(currentSpace, this.data.user) && this.data.user.ID != user.ID) {
            if (
              !window.confirm(
                "Are you sure you want to kick this user? This action cannot be undone."
              )
            )
              return;
            try {
              const response = await fetch(
                `/api/space_user/${currentSpace.UUID}`,
                {
                  method: "DELETE",
                  credentials: "include",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({
                    spaceUUID: currentSpace.UUID,
                    userID: user.ID,
                  }),
                }
              );
              if (response.ok) {
                // Handled by socket message remove-user with dashboard handleRemoveUser
              } else {
                const result = await response.json();
                window.alert(`ERROR: ${result.error}`);
              }
            } catch (error) {
              console.log(error);
            }
          }
        },
      }
    );
  };

  renderSpaceUsersList = (currentSpace) => {
    // Check selected space

    const elementList = [];

    if (currentSpace.Users) {
      // space users "invited"
      currentSpace.Users.map((user) => {
        elementList.push(this.renderUserElement(currentSpace, user));
      });
    }

    return elementList;
  };

  render = () => {
    this.domComponent.innerHTML = "";
    const currentSpaceUUID = this.getCurrentSpaceUUID();

    if (currentSpaceUUID) {
      const currentSpace = this.data.spaces.find(
        (space) => space.UUID === currentSpaceUUID
      );

      if (!currentSpace) return;

      this.domComponent.append(
        createElement("h3", {}, "Users"),
        ...this.renderSpaceUsersList(currentSpace)
      );
    }
  };
}

class SpaceItemComponent {
  constructor(space, user, openSpaceSettings, loadChannel) {
    this.space = space;
    this.user = user;
    this.openSpaceSettings = openSpaceSettings;
    this.loadChannel = loadChannel;
    this.channelsOpen = false;
    this.domComponent = createElement("div", { class: "space-item" });
    this.render();
  }

  isAuthor = () => {
    return String(this.space.AuthorID) === String(this.user.ID);
  };

  renderActions = () => {
    return [
      createElement("button", { class: "btn-icon open-settings" }, "⚙️", {
        type: "click",
        event: (e) => {
          e.stopPropagation();
          this.openSpaceSettings(this.space);
        },
      }),
    ];
  };

  renderChannelList = () => {
    return createElement(
      "div",
      { class: "channel-list" },
      (this.space.Channels || []).map((channel) =>
        createElement("div", { class: "channel-item" }, channel.Name, {
          type: "click",
          event: () => this.loadChannel(this.space.UUID, channel.UUID),
        })
      )
    );
  };

  toggleChannels = () => {
    this.channelsOpen = !this.channelsOpen;
    this.render();
  };

  render = () => {
    this.domComponent.innerHTML = "";
    this.domComponent.append(
      createElement(
        "div",
        { class: "space-header" },
        [
          createElement("span", {}, this.space.Name),
          createElement(
            "div",
            { class: "space-actions" },
            this.renderActions()
          ),
        ],
        { type: "click", event: this.toggleChannels }
      ),
      ...(this.channelsOpen ? [this.renderChannelList()] : [])
    );
  };
}
