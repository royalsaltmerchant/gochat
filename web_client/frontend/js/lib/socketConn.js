import { relayBaseURLWS } from "./config.js";
import platform from "../platform/index.js";
import identityManager from "./identityManager.js";

export default class SocketConn {
  constructor(props) {
    this.returnToHostList = props.returnToHostList;
    this.updateAccountUsername = props.updateAccountUsername;
    this.dashboardInitialRender = props.dashboardInitialRender;
    this.openDashModal = props.openDashModal;
    this.closeDashModal = props.closeDashModal;
    this.renderChatAppMessage = props.renderChatAppMessage;
    this.handleCreateSpace = props.handleCreateSpace;
    this.handleCreateChannel = props.handleCreateChannel;
    this.handleDeleteSpace = props.handleDeleteSpace;
    this.handleDeleteChannel = props.handleDeleteChannel;
    this.handleCreateChannelUpdate = props.handleCreateChannelUpdate;
    this.handleDeleteChannelUpdate = props.handleDeleteChannelUpdate;
    this.handleInviteUser = props.handleInviteUser;
    this.handleAddInvite = props.handleAddInvite;
    this.handleAcceptInvite = props.handleAcceptInvite;
    this.handleAcceptInviteUpdate = props.handleAcceptInviteUpdate;
    this.handleDeclineInvite = props.handleDeclineInvite;
    this.handleLeaveSpace = props.handleLeaveSpace;
    this.handleLeaveSpaceUpdate = props.handleLeaveSpaceUpdate;
    this.handleIncomingMessages = props.handleIncomingMessages;

    this.socket = null;
    this.manualClose = false;
    this.retryCount = 0;
    this.maxRetries = 2;

    this.hostUUID = localStorage.getItem("hostUUID");

    if (this.hostUUID) {
      this.connect();
    } else {
      platform.alert("Missing host key");
      this.returnToHostList();
    }
  }

  retryConnection = () => {
    const retryDelay = 2000;
    this.retryCount += 1;
    if (this.retryCount > this.maxRetries) {
      platform.alert("Unable to reconnect to host. Returning to home.");
      this.returnToHostList();
      return;
    }

    setTimeout(() => {
      if (this.socket?.readyState !== WebSocket.OPEN) {
        this.connect();
      }
    }, retryDelay);
  };

  connect = () => {
    this.socket = new WebSocket(relayBaseURLWS);

    this.socket.onopen = () => {
      this.joinHost();
    };

    this.socket.onerror = (error) => {
      console.error("WebSocket error:", error);
    };

    this.socket.onclose = () => {
      if (this.manualClose) return;
      this.retryConnection();
    };

    this.socket.onmessage = async (event) => {
      try {
        const data = JSON.parse(event.data);

        switch (data.type) {
          case "join_ack":
            break;
          case "join_error":
            platform.alert(data.data?.error || "Failed to join host");
            this.returnToHostList();
            break;
          case "auth_challenge":
            await this.authenticateWithPublicKey(data.data?.challenge);
            break;
          case "auth_pubkey_success":
            this.closeDashModal();
            this.getDashboardData();
            break;
          case "dash_data_payload":
            this.dashboardInitialRender(data.data);
            this.joinAllSpaces(data);
            break;
          case "update_username_success":
            this.updateAccountUsername(data, this.hostUUID);
            identityManager.setIdentityUsername(data.data.username);
            break;
          case "create_space_success":
            this.handleCreateSpace(data);
            break;
          case "delete_space_success":
            this.handleDeleteSpace();
            break;
          case "create_channel_success":
            this.handleCreateChannel(data);
            break;
          case "create_channel_update":
            this.handleCreateChannelUpdate(data);
            break;
          case "delete_channel_success":
            this.handleDeleteChannel(data);
            break;
          case "delete_channel_update":
            this.handleDeleteChannelUpdate(data);
            break;
          case "invite_user_success":
            this.handleInviteUser(data);
            break;
          case "invite_user_update":
            this.handleAddInvite(data);
            break;
          case "accept_invite_success":
            this.handleAcceptInvite(data);
            break;
          case "accept_invite_update":
            this.handleAcceptInviteUpdate(data);
            break;
          case "decline_invite_success":
            this.handleDeclineInvite(data);
            break;
          case "leave_space_success":
            this.handleLeaveSpace();
            break;
          case "leave_space_update":
            this.handleLeaveSpaceUpdate(data);
            break;
          case "chat":
            await this.renderChatAppMessage(data);
            break;
          case "get_messages_success":
            await this.handleIncomingMessages(data);
            break;
          case "error":
            platform.alert(data.data.error);
            break;
          case "authentication-error":
            platform.alert(data.data.error || "Authentication failed");
            break;
          case "author_error":
            platform.alert("Failed to connect to host. Host is offline.");
            this.returnToHostList();
            break;
          default:
            break;
        }
      } catch (err) {
        console.error("JSON parsing error:", err);
      }
    };
  };

  authenticateWithPublicKey = async (challenge) => {
    if (!challenge) {
      platform.alert("Missing auth challenge from server");
      return;
    }

    try {
      const identity = await identityManager.getOrCreateIdentity();
      const device = identityManager.getDeviceMetadata();
      const authMessage = `parch-chat-auth:${this.hostUUID}:${challenge}:${identity.encPublicKey}`;
      const signature = await identityManager.signAuthMessage(identity, authMessage);

      const wsMessage = {
        type: "auth_pubkey",
        data: {
          public_key: identity.publicKey,
          enc_public_key: identity.encPublicKey,
          device_id: device.deviceId,
          device_name: device.deviceName,
          username: identity.username || "",
          challenge,
          signature,
        },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } catch (error) {
      console.error(error);
      platform.alert("Failed to initialize public-key identity");
    }
  };

  joinHost = () => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(
        JSON.stringify({
          type: "join_host",
          data: {
            uuid: this.hostUUID,
          },
        })
      );
    }
  };

  getDashboardData = () => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "get_dash_data", data: "" }));
    }
  };

  joinAllSpaces = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      const uuids = data.data.spaces.map((space) => space.uuid);
      this.socket.send(
        JSON.stringify({
          type: "join_all_spaces",
          data: { space_uuids: uuids },
        })
      );
    }
  };

  updateUsername = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "update_username", data }));
    }
  };

  createSpace = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "create_space", data }));
    }
  };

  deleteSpace = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "delete_space", data }));
    }
  };

  createChannel = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "create_channel", data }));
    }
  };

  deleteChannel = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "delete_channel", data }));
    }
  };

  inviteUser = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "invite_user", data }));
    }
  };

  acceptInvite = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "accept_invite", data }));
    }
  };

  declineInvite = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "decline_invite", data }));
    }
  };

  leaveSpace = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "leave_space", data }));
    }
  };

  removeSpaceUser = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "remove_space_user", data }));
    }
  };

  joinChannel = (channelUUID) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "join_channel", data: { uuid: channelUUID } }));
    }
  };

  sendMessage = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({ type: "chat", data }));
    }
  };

  getMessages = (timestamp) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(
        JSON.stringify({
          type: "get_messages",
          data: { before_unix_time: timestamp },
        })
      );
    }
  };

  hardClose = () => {
    this.manualClose = true;
    this.close();
  };

  close = () => {
    if (this.socket) {
      this.socket.close();
    }
  };
}
