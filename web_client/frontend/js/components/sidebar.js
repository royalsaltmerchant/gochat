import { Component, createElement } from "../lib/foundation.js";
import platform from "../platform/index.js";

export default class SidebarComponent extends Component {
  constructor(props) {
    super({
      domElem: props.domElem || createElement("div", { class: "sidebar" }),
      autoInit: false,
      autoRender: props.autoRender,
    });

    this.data = props.data;
    this.socketConn = props.socketConn;
    this.returnToHostList = props.returnToHostList;
    this.openDashModal = props.openDashModal;
    this.closeDashModal = props.closeDashModal;
    this.getCurrentSpaceUUID = props.getCurrentSpaceUUID;
    this.openSpaceSettings = props.openSpaceSettings;
    this.loadChannel = props.loadChannel;

    this.spaceComponents = [];
    this.userAccountComponent = null;
    this.spaceUserListComponent = null;
    this._spaceChildKeys = new Set();
  }

  syncSpaceComponents = () => {
    const spaces = Array.isArray(this.data?.spaces) ? this.data.spaces : [];
    const nextSpaceKeys = new Set();

    this.spaceComponents = spaces.map((space) => {
      const childKey = `space:${space.uuid}`;
      nextSpaceKeys.add(childKey);

      return this.useChild(
        childKey,
        () =>
          new SpaceItemComponent({
            domElem: createElement("div", { class: "space-item" }),
            space,
            user: this.data.user,
            openSpaceSettings: this.openSpaceSettings,
            loadChannel: this.loadChannel,
            autoRender: false,
          }),
        (child) => {
          child.space = space;
          child.user = this.data.user;
          child.openSpaceSettings = this.openSpaceSettings;
          child.loadChannel = this.loadChannel;
        }
      );
    });

    for (const staleKey of this._spaceChildKeys) {
      if (!nextSpaceKeys.has(staleKey)) {
        this.dropChild(staleKey);
      }
    }

    this._spaceChildKeys = nextSpaceKeys;
    return this.spaceComponents;
  };

  addSpace = async (space) => {
    if (!Array.isArray(this.data.spaces)) {
      this.data.spaces = [];
    }
    this.data.spaces.push(space);
    await this.render();
  };

  render = async () => {
    const spaceComponents = this.syncSpaceComponents();
    await Promise.all(spaceComponents.map((comp) => comp.render()));

    const userAccountComponent = this.useChild(
      "user-account",
      () =>
        new UserAccountComponent({
          data: this.data,
          returnToHostList: this.returnToHostList,
          openDashModal: this.openDashModal,
          domElem: createElement("div", { class: "user-component" }),
          autoRender: false,
        }),
      (child) => {
        child.data = this.data;
        child.returnToHostList = this.returnToHostList;
        child.openDashModal = this.openDashModal;
      }
    );

    const spaceUserListComponent = this.useChild(
      "space-user-list",
      () =>
        new SpaceUserListComponent({
          data: this.data,
          socketConn: this.socketConn,
          getCurrentSpaceUUID: this.getCurrentSpaceUUID,
          domElem: createElement("div", {
            class: "space-users-list",
            id: "space-users-list",
          }),
          autoRender: false,
        }),
      (child) => {
        child.data = this.data;
        child.socketConn = this.socketConn;
        child.getCurrentSpaceUUID = this.getCurrentSpaceUUID;
      }
    );

    await userAccountComponent.render();
    await spaceUserListComponent.render();

    this.userAccountComponent = userAccountComponent;
    this.spaceUserListComponent = spaceUserListComponent;

    return [
      createElement("div", { class: "sidebar-spaces-container" }, [
        createElement(
          "div",
          { id: "spaces-list" },
          this.spaceComponents.map((comp) => comp.domElem)
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
                if (!value) return;
                this.socketConn.createSpace({
                  name: value.trim(),
                });
              });
            },
          }
        ),
      ]),
      createElement("br"),
      spaceUserListComponent.domElem,
      createElement("hr"),
      userAccountComponent.domElem,
    ];
  };
}

class UserAccountComponent extends Component {
  constructor(props) {
    super({
      domElem: props.domElem || createElement("div", { class: "user-component" }),
      autoInit: false,
      autoRender: props.autoRender,
    });
    this.data = props.data;
    this.returnToHostList = props.returnToHostList;
    this.openDashModal = props.openDashModal;
  }

  render = async () => {
    return [
      createElement("a", { href: "#" }, "Account", {
        type: "click",
        event: () => {
          this.openDashModal({
            type: "account",
            data: {
              user: this.data.user,
              active_devices: this.data.active_devices || [],
            },
          });
        },
      }),
      createElement("a", { href: "#" }, "Invites", {
        type: "click",
        event: () => {
          this.openDashModal({
            type: "invites",
            data: { invites: this.data.invites, user: this.data.user },
          });
        },
      }),
      createElement("a", { href: "#" }, "<- Host List", {
        type: "click",
        event: () => {
          this.returnToHostList();
        },
      }),
    ];
  };
}

class SpaceUserListComponent extends Component {
  constructor(props) {
    super({
      domElem:
        props.domElem || createElement("div", { class: "space-users-list", id: "space-users-list" }),
      autoInit: false,
      autoRender: props.autoRender,
    });
    this.data = props.data;
    this.socketConn = props.socketConn;
    this.getCurrentSpaceUUID = props.getCurrentSpaceUUID;
  }

  isAuthor = (space, user) => {
    return String(space.author_id) === String(user.id);
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
            this.data.user.id !== user.id
          ) {
            platform.confirm("Are you sure you want to remove this user?").then((confirmed) => {
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
    const elementList = [];

    if (Array.isArray(currentSpace?.users)) {
      currentSpace.users.forEach((user) => {
        elementList.push(this.renderUserElement(currentSpace, user));
      });
    }

    return elementList;
  };

  render = async () => {
    const currentSpaceUUID = this.getCurrentSpaceUUID();

    if (!currentSpaceUUID) {
      return [];
    }

    const currentSpace = (this.data?.spaces || []).find(
      (space) => space.uuid === currentSpaceUUID
    );

    if (!currentSpace) {
      return [];
    }

    return [createElement("h3", {}, "Users"), ...this.renderSpaceUsersList(currentSpace)];
  };
}

class SpaceItemComponent extends Component {
  constructor(props) {
    super({
      domElem: props.domElem || createElement("div", { class: "space-item" }),
      state: {
        channelsOpen: false,
      },
      autoInit: false,
      autoRender: props.autoRender,
    });

    this.space = props.space;
    this.user = props.user;
    this.openSpaceSettings = props.openSpaceSettings;
    this.loadChannel = props.loadChannel;
  }

  isAuthor = () => {
    return String(this.space.author_id) === String(this.user.id);
  };

  toggleChannels = async () => {
    await this.setState((prev) => ({ channelsOpen: !prev.channelsOpen }));
  };

  renderChannelList = () => {
    const channels = Array.isArray(this.space?.channels) ? this.space.channels : [];

    const channelNodes = this.useMemo(
      "channel-nodes",
      () =>
        channels.map((channel) =>
          createElement(
            "div",
            { class: "channel-item" },
            [
              createElement("span", { class: "channel-hash" }, "#"),
              createElement("span", {}, channel.name),
            ],
            {
              type: "click",
              event: () => this.loadChannel(this.space.uuid, channel.uuid),
            }
          )
        ),
      () => channels.map((channel) => `${channel.uuid}:${channel.name}`)
    );

    return createElement("div", { class: "channel-list" }, channelNodes);
  };

  render = async () => {
    this.domElem.className = `space-item${this.state.channelsOpen ? " space-item--open" : ""}`;

    return [
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
      ...(this.state.channelsOpen ? [this.renderChannelList()] : []),
    ];
  };
}
