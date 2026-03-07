import { Component, createElement } from "./lib/foundation.js";
import DashModal from "./components/dashModal.js";
import SidebarComponent from "./components/sidebar.js";
import MainContentComponent from "./components/mainContent.js";
import SocketConn from "./lib/socketConn.js";
import identityManager from "./lib/identityManager.js";
import e2ee from "./lib/e2ee.js";

export default class DashboardApp extends Component {
  constructor(props) {
    super({
      domElem: props.domElem || createElement("div", { class: "dashboard-container" }),
      state: {
        status: "connecting",
        data: null,
      },
      autoInit: props.autoInit,
      autoRender: props.autoRender,
    });
    this.returnToHostList = props.returnToHostList;

    this.data = null;
    this.sidebar = null;
    this.dashModal = null;
    this.mainContent = null;
    this.currentSpaceUUID = null;
    this.socketConn = null;
  }

  init = async () => {
    if (this.socketConn) {
      return;
    }

    this.socketConn = new SocketConn({
      returnToHostList: this.returnToHostList,
      updateAccountUsername: this.updateAccountUsername,
      dashboardInitialRender: this.initialRender,
      openDashModal: this.openDashModal,
      closeDashModal: this.closeDashModal,
      renderChatAppMessage: this.renderChatAppMessage,
      handleCreateSpace: this.handleCreateSpace,
      handleDeleteSpace: this.handleDeleteSpace,
      handleCreateChannel: this.handleCreateChannel,
      handleCreateChannelUpdate: this.handleCreateChannelUpdate,
      handleDeleteChannel: this.handleDeleteChannel,
      handleDeleteChannelUpdate: this.handleDeleteChannelUpdate,
      handleInviteUser: this.handleInviteUser,
      handleAddInvite: this.handleAddInvite,
      handleAcceptInvite: this.handleAcceptInvite,
      handleAcceptInviteUpdate: this.handleAcceptInviteUpdate,
      handleDeclineInvite: this.handleDeclineInvite,
      handleLeaveSpace: this.handleLeaveSpace,
      handleLeaveSpaceUpdate: this.handleLeaveSpaceUpdate,
      handleIncomingMessages: this.handleIncomingMessages,
    });

    this.onCleanup(() => {
      this.socketConn?.hardClose?.();
      this.socketConn = null;
    });

    this.ensureDashModal();
  };

  ensureDashModal = () => {
    const modal = this.useChild(
      "dash-modal",
      () => new DashModal(this, this.socketConn),
      (child) => {
        child.app = this;
        child.socketConn = this.socketConn;
      }
    );
    this.dashModal = modal;
    return modal;
  };

  initialRender = async (data) => {
    const normalizedData = {
      ...data,
      spaces: Array.isArray(data?.spaces) ? data.spaces : [],
      active_devices: Array.isArray(data?.active_devices) ? data.active_devices : [],
    };

    this.data = normalizedData;

    await this.setState({
      status: "ready",
      data: normalizedData,
    });
  };

  updateAccountUsername = (data) => {
    if (!this.data?.user) return;

    this.data.user.username = data.data.username;
    this.sidebar?.userAccountComponent?.render?.();
    this.openDashModal({
      type: "account",
      data: { user: this.data.user, active_devices: this.data.active_devices || [] },
    });
  };

  getCurrentSpaceUUID = () => {
    return this.currentSpaceUUID;
  };

  openDashModal = (props) => {
    return this.ensureDashModal().open(props);
  };

  closeDashModal = () => {
    this.ensureDashModal().close();
  };

  openSpaceSettings = (space) => {
    this.currentSpaceUUID = space.uuid;
    this.openDashModal({
      type: "space-settings",
      data: { space: space, user: this.data.user },
    });
  };

  closeSpaceSettings = () => {
    this.closeDashModal();
    this.currentSpaceUUID = null;
  };

  invalidateChannelSpaceMemo = () => {
    this.clearMemo("channel-space-map");
  };

  loadChannel = (spaceUUID, channelUUID) => {
    this.currentSpaceUUID = spaceUUID;

    this.socketConn.joinChannel(spaceUUID, channelUUID);
    this.sidebar.spaceUserListComponent.render();
    this.mainContent.renderChannel(channelUUID);
  };

  handleCreateSpace = async (data) => {
    if (!this.data) return;

    this.data.spaces.push(data.data.space);
    this.invalidateChannelSpaceMemo();

    await this.render();
  };

  handleDeleteSpace = async () => {
    if (!this.currentSpaceUUID || !this.data) return;

    this.data.spaces = this.data.spaces.filter(
      (space) => space.uuid !== this.currentSpaceUUID
    );

    this.currentSpaceUUID = null;
    this.invalidateChannelSpaceMemo();
    this.mainContent?.cleanup?.();

    this.closeSpaceSettings();
    await this.render();
  };

  handleInviteUser = () => {
    window.alert("Invite sent");
  };

  handleAddInvite = (data) => {
    if (!this.data) return;

    if (!this.data.invites) {
      this.data.invites = [];
    }
    this.data.invites.push(data.data.invite);
  };

  handleAcceptInvite = async (data) => {
    if (!this.data) return;

    this.data.invites = this.data.invites.filter(
      (invite) => invite.id !== data.data.space_user_id
    );
    this.data.spaces.push(data.data.space);
    this.invalidateChannelSpaceMemo();

    this.openDashModal({
      type: "invites",
      data: { invites: this.data.invites, user: this.data.user },
    });
    await this.render();
  };

  handleAcceptInviteUpdate = (data) => {
    if (!this.data) return;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === data.data.space_uuid
    );

    if (!spaceToUpdate) return;

    if (
      !spaceToUpdate.users.find(
        (user) =>
          user.id === data.data.user.id ||
          (user.public_key &&
            data.data.user.public_key &&
            user.public_key === data.data.user.public_key)
      )
    ) {
      spaceToUpdate.users.push(data.data.user);
    }

    if (this.currentSpaceUUID === spaceToUpdate.uuid && this.data.user.id !== data.data.user.id) {
      this.sidebar.spaceUserListComponent.render();
    }
  };

  handleDeclineInvite = (data) => {
    if (!this.data) return;

    this.data.invites = this.data.invites.filter(
      (invite) => invite.id !== data.data.space_user_id
    );
    this.openDashModal({
      type: "invites",
      data: { invites: this.data.invites, user: this.data.user },
    });
  };

  handleLeaveSpace = async () => {
    if (!this.data) return;

    this.data.spaces = this.data.spaces.filter(
      (space) => space.uuid !== this.currentSpaceUUID
    );
    this.currentSpaceUUID = null;
    this.invalidateChannelSpaceMemo();
    this.mainContent?.cleanup?.();

    await this.render();
  };

  handleLeaveSpaceUpdate = (data) => {
    if (!this.data) return;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === data.data.space_uuid
    );

    if (spaceToUpdate && this.currentSpaceUUID === spaceToUpdate.uuid) {
      const indexOfUser = spaceToUpdate.users.findIndex(
        (user) =>
          user.id === data.data.user_id ||
          (user.public_key &&
            data.data.user_public_key &&
            user.public_key === data.data.user_public_key)
      );
      if (indexOfUser >= 0) {
        spaceToUpdate.users.splice(indexOfUser, 1);
        this.sidebar.spaceUserListComponent.render();
      }
    }
  };

  handleCreateChannel = (data) => {
    if (!this.data) return;

    const { space_uuid: spaceUUID, channel } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    spaceToUpdate.channels.push(channel);
    this.invalidateChannelSpaceMemo();

    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();

    this.openDashModal({
      type: "space-settings",
      data: { space: spaceToUpdate, user: this.data.user },
    });
  };

  handleCreateChannelUpdate = (data) => {
    if (!this.data) return;

    const { space_uuid: spaceUUID, channel } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    const alreadyExists = spaceToUpdate.channels.some(
      (existingChannel) => existingChannel.uuid === channel.uuid
    );
    if (alreadyExists) return;

    spaceToUpdate.channels.push(channel);
    this.invalidateChannelSpaceMemo();

    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();
  };

  handleDeleteChannel = (data) => {
    if (!this.data) return;

    const { uuid: deletedChannelUUID, space_uuid: spaceUUID } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    const channelIndex = spaceToUpdate.channels.findIndex(
      (channel) => channel.uuid === deletedChannelUUID
    );

    if (channelIndex === -1) return;

    spaceToUpdate.channels.splice(channelIndex, 1);
    this.invalidateChannelSpaceMemo();

    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();

    this.openDashModal({
      type: "space-settings",
      data: { space: spaceToUpdate, user: this.data.user },
    });

    if (this.mainContent.currentChannelUUID === deletedChannelUUID) {
      this.mainContent.cleanup?.();
      this.mainContent.render();
    }
  };

  handleDeleteChannelUpdate = (data) => {
    if (!this.data) return;

    const { uuid: deletedChannelUUID, space_uuid: spaceUUID } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    const channelIndex = spaceToUpdate.channels.findIndex(
      (channel) => channel.uuid === deletedChannelUUID
    );
    if (channelIndex === -1) return;

    spaceToUpdate.channels.splice(channelIndex, 1);
    this.invalidateChannelSpaceMemo();

    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();

    if (this.mainContent.currentChannelUUID === deletedChannelUUID) {
      this.mainContent.cleanup?.();
      this.mainContent.render();
    }
  };

  findSpaceByChannelUUID = (channelUUID) => {
    if (!this.data?.spaces || !channelUUID) return null;

    const channelSpaceMap = this.useMemo(
      "channel-space-map",
      () => {
        const map = new Map();
        for (const space of this.data?.spaces || []) {
          for (const channel of space.channels || []) {
            map.set(channel.uuid, space);
          }
        }
        return map;
      },
      () => {
        const deps = [];
        for (const space of this.data?.spaces || []) {
          deps.push(space.uuid);
          for (const channel of space.channels || []) {
            deps.push(channel.uuid);
          }
        }
        return deps;
      }
    );

    return channelSpaceMap.get(channelUUID) || null;
  };

  resolveUsernameForAuthPublicKey = (authPublicKey, spaceUUID) => {
    if (!authPublicKey || !this.data?.spaces) return "unknown";

    const lookupSpaces = spaceUUID
      ? this.data.spaces.filter((space) => space.uuid === spaceUUID)
      : this.data.spaces;

    for (const space of lookupSpaces) {
      const matched = (space.users || []).find(
        (user) => user.public_key === authPublicKey
      );
      if (matched?.username) {
        return matched.username;
      }
    }

    return "unknown";
  };

  decryptWireMessage = async (wireMessage, spaceUUID) => {
    const identity = await identityManager.getOrCreateIdentity();
    const envelope = wireMessage?.envelope;
    if (!envelope) {
      throw new Error("Missing encrypted envelope");
    }
    const content = await e2ee.decryptMessageForIdentity({
      envelope,
      identity,
    });

    const senderAuthPublicKey = envelope.sender_auth_public_key || "";
    return {
      ...wireMessage,
      content,
      sender_auth_public_key: senderAuthPublicKey,
      username: this.resolveUsernameForAuthPublicKey(senderAuthPublicKey, spaceUUID),
    };
  };

  renderChatAppMessage = async (data) => {
    if (!this.mainContent?.chatApp) {
      return;
    }

    try {
      const decryptedMessage = await this.decryptWireMessage(
        data.data,
        this.currentSpaceUUID
      );
      const messageComponent =
        this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent;

      messageComponent.appendNewMessage(decryptedMessage);
      if (
        decryptedMessage.sender_auth_public_key === this.data.user.public_key
      ) {
        messageComponent.scrollDown();
      } else if (messageComponent.isScrolledToBottom()) {
        messageComponent.scrollDown();
      }
    } catch (err) {
      console.error(err);
    }
  };

  handleIncomingMessages = async (data) => {
    if (
      !data?.data?.messages ||
      !this.mainContent?.chatApp ||
      this.mainContent.chatApp.chatBoxComponent.channelUUID !== data.data.channel_uuid
    ) {
      return;
    }

    const component = this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent;
    const container = component.domElem;

    const previousHeight = container.scrollHeight;
    const previousScrollTop = container.scrollTop;

    component.hasMoreMessages = data.data.has_more_messages;
    component.isLoading = false;

    const space = this.findSpaceByChannelUUID(data.data.channel_uuid);
    const decryptedMessages = await Promise.all(
      data.data.messages.map(async (message) => {
        try {
          return await this.decryptWireMessage(message, space?.uuid || null);
        } catch (err) {
          console.error(err);
          return {
            ...message,
            content: "[Unable to decrypt message]",
            username: "unknown",
            sender_auth_public_key: "",
          };
        }
      })
    );

    component.chatBoxMessages = [...decryptedMessages, ...component.chatBoxMessages];

    component.render();

    const newHeight = container.scrollHeight;
    container.scrollTop = newHeight - previousHeight + previousScrollTop;
  };

  render = async () => {
    const modal = this.ensureDashModal();

    if (!this.socketConn || this.state.status !== "ready" || !this.state.data) {
      return [
        createElement("div", { class: "initial-spinner" }, [
          createElement("div", {}, "Connecting To Host..."),
          createElement("br"),
          createElement("div", { class: "lds-dual-ring" }),
        ]),
        modal.domElem,
      ];
    }

    this.data = this.state.data;

    const sidebar = this.useChild(
      "sidebar",
      () =>
        new SidebarComponent({
          data: this.data,
          socketConn: this.socketConn,
          returnToHostList: this.returnToHostList,
          domElem: createElement("div", { class: "sidebar" }),
          openDashModal: this.openDashModal,
          closeDashModal: this.closeDashModal,
          getCurrentSpaceUUID: this.getCurrentSpaceUUID,
          openSpaceSettings: this.openSpaceSettings,
          loadChannel: this.loadChannel,
          autoRender: false,
        }),
      (child) => {
        child.data = this.data;
        child.socketConn = this.socketConn;
        child.returnToHostList = this.returnToHostList;
        child.openDashModal = this.openDashModal;
        child.closeDashModal = this.closeDashModal;
        child.getCurrentSpaceUUID = this.getCurrentSpaceUUID;
        child.openSpaceSettings = this.openSpaceSettings;
        child.loadChannel = this.loadChannel;
      }
    );

    const mainContent = this.useChild(
      "main-content",
      () =>
        new MainContentComponent({
          socketConn: this.socketConn,
          data: this.data,
          domElem: createElement("div", {
            id: "channel-content",
            class: "channel-content",
          }),
          autoRender: false,
        }),
      (child) => {
        child.socketConn = this.socketConn;
        child.data = this.data;
      }
    );

    this.sidebar = sidebar;
    this.mainContent = mainContent;

    await sidebar.render();
    await mainContent.render();

    return [sidebar.domElem, mainContent.domElem, modal.domElem];
  };
}
