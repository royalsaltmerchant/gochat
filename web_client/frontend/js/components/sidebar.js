import createElement from "./createElement.js";
import platform from "../platform/index.js";

export default class SidebarComponent {
  constructor(props) {
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.returnToHostList = props.returnToHostList;
    this.openDashModal = props.openDashModal;
    this.closeDashModal = props.closeDashModal;
    this.getCurrentSpaceUUID = props.getCurrentSpaceUUID;
    this.spaceComponents = [];
    this.domComponent = props.domComponent;
    this.openSpaceSettings = props.openSpaceSettings;
    this.loadChannel = props.loadChannel;

    this.render();
  }

  initSpaceComponents = () => {
    if (this.data.spaces) {
      this.spaceComponents = this.data.spaces.map(
        (space) =>
          new SpaceItemComponent({
            domComponent: createElement("div", { class: "space-item" }),
            space: space,
            user: this.data.user,
            openSpaceSettings: this.openSpaceSettings,
            loadChannel: this.loadChannel,
          })
      );
    }
  };

  addSpace = (space) => {
    const comp = new SpaceItemComponent({
      domComponent: createElement("div", { class: "space-item" }),
      space: space,
      user: this.data.user,
      openSpaceSettings: this.openSpaceSettings,
      loadChannel: this.loadChannel,
    });
    this.spaceComponents.push(comp);
    this.render();
  };

  render = () => {
    this.domComponent.innerHTML = "";

    this.initSpaceComponents();

    this.userAccountComponent = new UserAccountComponent({
      data: this.data,
      returnToHostList: this.returnToHostList,
      openDashModal: this.openDashModal,
      domComponent: createElement("div", { class: "user-component" }),
    });

    this.spaceUserListComponent = new SpaceUserListComponent({
      data: this.data,
      socketConn: this.socketConn,
      getCurrentSpaceUUID: this.getCurrentSpaceUUID,
      domComponent: createElement("div", {
        class: "space-users-list",
        id: "space-users-list",
      }),
    });

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
          {
            type: "click",
            event: () => {
              this.openDashModal({
                type: "prompt",
                data: { message: "Please enter Space 'Name'" },
              }).then((value) => {
                if (value) {
                  value.trim();
                  this.socketConn.createSpace({
                    name: value,
                  });
                }
              });
            },
          }
        ),
      ]),
      createElement("br"),
      this.spaceUserListComponent.domComponent,
      createElement("hr"),
      this.userAccountComponent.domComponent
    );
  };
}

class UserAccountComponent {
  constructor(props) {
    this.data = props.data;
    this.returnToHostList = props.returnToHostList;
    this.openDashModal = props.openDashModal;
    this.domComponent = props.domComponent;

    this.render();
  }

  render = () => {
    this.domComponent.innerHTML = "";

    this.domComponent.append(
      createElement("a", { href: "#" }, "Account", {
        type: "click",
        event: (e) => {
          this.openDashModal({
            type: "account",
            data: { user: this.data.user },
          });
        },
      }),
      createElement("a", { href: "#" }, "Invites", {
        type: "click",
        event: (e) => {
          this.openDashModal({
            type: "invites",
            data: { invites: this.data.invites, user: this.data.user },
          });
        },
      }),
      createElement("a", { href: "#" }, "<- Host List", {
        type: "click",
        event: (e) => {
          this.returnToHostList();
        },
      })
    );
  };
}

class SpaceUserListComponent {
  constructor(props) {
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.getCurrentSpaceUUID = props.getCurrentSpaceUUID;
    this.domComponent = props.domComponent;

    this.render();
  }

  isAuthor = (space, user) => {
    return String(space.author_id) === String(user.id); // the current user not space user
  };

  renderUserElement = (currentSpace, user) => {
    return createElement(
      "div",
      { class: "space-user-item", id: "space-user-item" },
      `${user.username} ${this.isAuthor(currentSpace, user) ? "*" : ""}`,
      {
        type: "click",
        event: async () => {
          if (
            this.isAuthor(currentSpace, this.data.user) &&
            this.data.user.id != user.id
          ) {
            platform.confirm(
              "Are you sure you want to remove this user?"
            ).then((confirmed) => {
              if (confirmed) {
                this.socketConn.removeSpaceUser({
                  space_uuid: currentSpace.uuid,
                  user_id: user.id,
                  user_public_key: user.public_key || "",
                });
              }
            });
          }
        },
      }
    );
  };

  renderSpaceUsersList = (currentSpace) => {
    // Check selected space

    const elementList = [];

    if (currentSpace.users) {
      // space users "invited"
      currentSpace.users.map((user) => {
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
        (space) => space.uuid === currentSpaceUUID
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
  constructor(props) {
    this.space = props.space;
    this.user = props.user;
    this.openSpaceSettings = props.openSpaceSettings;
    this.loadChannel = props.loadChannel;
    this.channelsOpen = false;
    this.domComponent = props.domComponent;
    this.render();
  }

  isAuthor = () => {
    return String(this.space.author_id) === String(this.user.id);
  };

  renderChannelList = () => {
    return createElement(
      "div",
      { class: "channel-list" },
      (this.space.channels || []).map((channel) =>
        createElement("div", { class: "channel-item" }, [
          createElement("span", { class: "channel-hash" }, "#"),
          createElement("span", {}, channel.name),
        ], {
          type: "click",
          event: () => this.loadChannel(this.space.uuid, channel.uuid),
        })
      )
    );
  };

  toggleChannels = () => {
    this.channelsOpen = !this.channelsOpen;
    this.render();
  };

  render = () => {
    this.domComponent.textContent = "";
    this.domComponent.className = `space-item${this.channelsOpen ? " space-item--open" : ""}`;
    this.domComponent.append(
      createElement(
        "div",
        { class: "space-header" },
        [
          createElement("span", { class: "space-chevron" }),
          createElement("span", {}, this.space.name),
          createElement(
            "div",
            { class: "space-actions" },
            createElement("button", { class: "btn-icon open-settings" }, "⚙️", {
              type: "click",
              event: (e) => {
                e.stopPropagation();
                this.openSpaceSettings(this.space);
              },
            })
          ),
        ],
        { type: "click", event: this.toggleChannels }
      ),
      ...(this.channelsOpen ? [this.renderChannelList()] : [])
    );
  };
}
