import createElement from "./components/createElement.js";
import DashModal from "./components/dashModal.js";
import SidebarComponent from "./components/sidebar.js";
import MainContentComponent from "./components/mainContent.js";
import SocketConn from "./lib/socketConn.js";
import identityManager from "./lib/identityManager.js";
import e2ee from "./lib/e2ee.js";

export default class DashboardApp {
  constructor(props) {
    this.domComponent = createElement("div", { class: "dashboard-container" });
    this.returnToHostList = props.returnToHostList;
    this.data = null;
    this.sidebar = null;
    this.dashModal = null;
    this.mainContent = null;
    this.currentSpaceUUID = null;
    this.socketConn = null;
  }

  initialize = () => {
    // Render the spinner until socket calls render full page
    this.domComponent.append(
      createElement("div", { class: "initial-spinner" }, [
        createElement("div", {}, "Connecting To Host..."),
        createElement("br"),
        createElement("div", { class: "lds-dual-ring" }),
      ])
    );

    // Start socket conn
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

    // Render modal container early for account/invite/settings dialogs
    this.dashModal = new DashModal(this, this.socketConn);
    this.domComponent.append(this.dashModal.domComponent);
  };

  initialRender = (data) => {
    // Expected to receive data from socket
    this.data = data;
    if (!this.data.spaces) {
      this.data.spaces = []; // init spaces
    }

    this.sidebar = new SidebarComponent({
      data: this.data,
      socketConn: this.socketConn,
      returnToHostList: this.returnToHostList,
      domComponent: createElement("div", { class: "sidebar" }),
      openDashModal: this.openDashModal,
      closeDashModal: this.closeDashModal,
      getCurrentSpaceUUID: this.getCurrentSpaceUUID,
      openSpaceSettings: this.openSpaceSettings,
      loadChannel: this.loadChannel,
    });
    this.mainContent = new MainContentComponent({
      socketConn: this.socketConn,
      data: this.data,
      domComponent: createElement("div", {
        id: "channel-content",
        class: "channel-content",
      }),
    });

    this.render();
  };

  updateAccountUsername = (data, hostUUID) => {
    this.data.user.username = data.data.username;
    this.sidebar.userAccountComponent.render();
    this.openDashModal({ type: "account", data: { user: this.data.user } });
  };

  getCurrentSpaceUUID = () => {
    return this.currentSpaceUUID;
  };

  loadChannel = (spaceUUID, channelUUID) => {
    this.currentSpaceUUID = spaceUUID;

    this.socketConn.joinChannel(channelUUID); // Join the socket to the channel
    this.sidebar.spaceUserListComponent.render(); // Update the users list
    this.mainContent.renderChannel(channelUUID);
  };

  openDashModal = (props) => {
    return this.dashModal.open(props);
  };

  closeDashModal = () => {
    this.dashModal.close();
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

  handleCreateSpace = async (data) => {
    this.data.spaces.push(data.data.space);
    this.sidebar.render();
  };

  handleDeleteSpace = async () => {
    if (!this.currentSpaceUUID) return;

    this.data.spaces = this.data.spaces.filter(
      (s) => s.uuid !== this.currentSpaceUUID
    );

    const remainingComponents = this.sidebar.spaceComponents.filter(
      (comp) => comp.space.uuid !== this.currentSpaceUUID
    );
    this.sidebar.spaceComponents = remainingComponents;

    this.currentSpaceUUID = null;
    this.mainContent.render();
    this.sidebar.render();
    this.sidebar.spaceUserListComponent.render();
    this.closeSpaceSettings();
  };

  handleInviteUser = (data) => {
    window.alert("Invite sent");
  };

  handleAddInvite = (data) => {
    if (!this.data.invites) {
      this.data.invites = [];
    }
    this.data.invites.push(data.data.invite);
  };

  handleAcceptInvite = (data) => {
    this.data.invites = this.data.invites.filter(
      (i) => i.id !== data.data.space_user_id
    );
    this.data.spaces.push(data.data.space);
    this.sidebar.render();
    this.openDashModal({
      type: "invites",
      data: { invites: this.data.invites, user: this.data.user },
    });
  };

  handleAcceptInviteUpdate = (data) => {
    console.log("invite update", data);
    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === data.data.space_uuid
    );

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

    if (spaceToUpdate && this.currentSpaceUUID === spaceToUpdate.uuid) {
      if (this.data.user.id !== data.data.user.id) {
        this.sidebar.spaceUserListComponent.render();
      }
    }
  };

  handleDeclineInvite = (data) => {
    this.data.invites = this.data.invites.filter(
      (i) => i.id !== data.data.space_user_id
    );
    this.openDashModal({
      type: "invites",
      data: { invites: this.data.invites, user: this.data.user },
    });
  };

  handleLeaveSpace = () => {
    this.data.spaces = this.data.spaces.filter(
      (s) => s.uuid !== this.currentSpaceUUID
    );
    this.currentSpaceUUID = null;
    this.sidebar.render();
    this.mainContent.render();
  };

  handleLeaveSpaceUpdate = (data) => {
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
    const { space_uuid: spaceUUID, channel } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    // Add the new channel
    spaceToUpdate.channels.push(channel);

    // Re-render the corresponding sidebar element
    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();

    // Open the updated modal
    this.openDashModal({
      type: "space-settings",
      data: { space: spaceToUpdate, user: this.data.user },
    });
  };

  handleCreateChannelUpdate = (data) => {
    const { space_uuid: spaceUUID, channel } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    // Only add if not already present
    const alreadyExists = spaceToUpdate.channels.some(
      (chan) => chan.uuid === channel.uuid
    );
    if (alreadyExists) return;

    spaceToUpdate.channels.push(channel);

    // Re-render the corresponding sidebar element
    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();
  };

  handleDeleteChannel = (data) => {
    const { uuid: deletedChannelUUID, space_uuid: spaceUUID } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    const channelIndex = spaceToUpdate.channels.findIndex(
      (channel) => channel.uuid === deletedChannelUUID
    );

    if (channelIndex === -1) return;

    // Remove the channel
    spaceToUpdate.channels.splice(channelIndex, 1);

    // Re-render the affected space in the sidebar
    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();

    // Update the dashboard modal
    this.openDashModal({
      type: "space-settings",
      data: { space: spaceToUpdate, user: this.data.user },
    });

    // If the deleted channel was currently open, reset the main view
    if (this.mainContent.currentChannelUUID === deletedChannelUUID) {
      this.mainContent.currentChannelUUID = null;
      this.mainContent.chatApp = null;
      this.mainContent.render();
    }
  };

  handleDeleteChannelUpdate = (data) => {
    const { uuid: deletedChannelUUID, space_uuid: spaceUUID } = data.data;

    const spaceToUpdate = this.data.spaces.find(
      (space) => space.uuid === spaceUUID
    );
    if (!spaceToUpdate) return;

    const channelIndex = spaceToUpdate.channels.findIndex(
      (channel) => channel.uuid === deletedChannelUUID
    );
    if (channelIndex === -1) return;

    // Remove the channel
    spaceToUpdate.channels.splice(channelIndex, 1);

    // Re-render the affected space in the sidebar
    const spaceElem = this.sidebar.spaceComponents.find(
      (elem) => elem.space.uuid === spaceUUID
    );
    if (spaceElem) spaceElem.render();

    // If the deleted channel was currently open, reset the main view
    if (this.mainContent.currentChannelUUID === deletedChannelUUID) {
      this.mainContent.currentChannelUUID = null;
      this.mainContent.chatApp = null;
      this.mainContent.render();
    }
  };

  findSpaceByChannelUUID = (channelUUID) => {
    if (!this.data?.spaces || !channelUUID) return null;
    return (
      this.data.spaces.find((space) =>
        (space.channels || []).some((channel) => channel.uuid === channelUUID)
      ) || null
    );
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
    if (this.mainContent.chatApp) {
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
    }
  };

  handleIncomingMessages = async (data) => {
    if (
      data.data.messages &&
      this.mainContent.chatApp &&
      this.mainContent.chatApp.chatBoxComponent.channelUUID ===
        data.data.channel_uuid
    ) {
      const component =
        this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent;
      const container = component.domComponent;

      // Capture scroll position before prepending
      const previousHeight = container.scrollHeight;
      const previousScrollTop = container.scrollTop;

      // Update flags
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

      // Prepend older messages
      component.chatBoxMessages = [
        ...decryptedMessages,
        ...component.chatBoxMessages,
      ];

      // Re-render updated messages
      component.render();

      // Restore scroll position to maintain visual position
      const newHeight = container.scrollHeight;
      container.scrollTop = newHeight - previousHeight + previousScrollTop;
    }
  };

  render() {
    this.domComponent.innerHTML = "";

    this.domComponent.append(
      this.sidebar.domComponent,
      this.mainContent.domComponent,
      this.dashModal.domComponent
    );
  }
}
