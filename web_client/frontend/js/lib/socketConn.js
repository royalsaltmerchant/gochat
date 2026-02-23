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
    this.capabilityBySpaceUUID = new Map();
    this.hasInitialDashboardData = false;
    this.capabilityRefreshPromise = null;
    this.capabilityRefreshResolve = null;
    this.capabilityRefreshReject = null;
    this.capabilityRefreshTimer = null;
    this.lastCapabilityAction = null;
    this.capabilitySkewMs = 30 * 1000;
    this.capabilityRefreshTimeoutMs = 7000;
    this.capabilityRetryWindowMs = 10 * 1000;

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
            this.syncCapabilities(data.data);
            this.resolveCapabilityRefreshRequest();
            if (!this.hasInitialDashboardData) {
              this.hasInitialDashboardData = true;
              this.dashboardInitialRender(data.data);
              this.joinAllSpaces(data);
            }
            break;
          case "update_username_success":
            this.updateAccountUsername(data, this.hostUUID);
            identityManager.setIdentityUsername(data.data.username);
            break;
          case "create_space_success":
            this.setCapability(data.data?.capability);
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
            this.setCapability(data.data?.capability);
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
            if (!this.retryCapabilityActionFromError(data.data?.error || "")) {
              platform.alert(data.data.error);
            }
            break;
          case "authentication-error":
            platform.alert(data.data.error || "Authentication failed");
            break;
          case "author_error":
            platform.alert("Failed to connect to host. Host is offline.");
            this.rejectCapabilityRefreshRequest(new Error("host unavailable"));
            this.returnToHostList();
            break;
          default:
            break;
        }
      } catch (err) {
        console.error("JSON parsing error:", err);
      }
    };

    this.socket.onclose = () => {
      this.rejectCapabilityRefreshRequest(new Error("socket closed"));
      if (this.manualClose) return;
      this.retryConnection();
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

  syncCapabilities = (payload = {}) => {
    const capabilities = payload?.capabilities || [];
    this.capabilityBySpaceUUID.clear();
    capabilities.forEach((capability) => this.setCapability(capability));
    this.scheduleCapabilityRefresh();
  };

  setCapability = (capability) => {
    if (!capability || !capability.space_uuid || !capability.token) {
      return;
    }
    this.capabilityBySpaceUUID.set(capability.space_uuid, capability);
    this.scheduleCapabilityRefresh();
  };

  getCapabilityToken = (spaceUUID) => {
    if (!spaceUUID) {
      return "";
    }
    const capability = this.capabilityBySpaceUUID.get(spaceUUID);
    if (!capability || !capability.token) {
      return "";
    }
    return capability.token;
  };

  hasFreshCapability = (spaceUUID) => {
    const capability = this.capabilityBySpaceUUID.get(spaceUUID);
    if (!capability || !capability.token) {
      return false;
    }
    const expiresAtMs = Number(capability.expires_at || 0) * 1000;
    if (!Number.isFinite(expiresAtMs) || expiresAtMs <= 0) {
      return false;
    }
    return expiresAtMs > Date.now() + this.capabilitySkewMs;
  };

  resolveCapabilityRefreshRequest = () => {
    if (!this.capabilityRefreshResolve) return;
    const resolve = this.capabilityRefreshResolve;
    this.capabilityRefreshPromise = null;
    this.capabilityRefreshResolve = null;
    this.capabilityRefreshReject = null;
    resolve();
  };

  rejectCapabilityRefreshRequest = (error) => {
    if (!this.capabilityRefreshReject) return;
    const reject = this.capabilityRefreshReject;
    this.capabilityRefreshPromise = null;
    this.capabilityRefreshResolve = null;
    this.capabilityRefreshReject = null;
    reject(error);
  };

  requestCapabilityRefresh = () => {
    if (this.capabilityRefreshPromise) {
      return this.capabilityRefreshPromise;
    }
    if (this.socket?.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error("socket not connected"));
    }
    this.capabilityRefreshPromise = new Promise((resolve, reject) => {
      this.capabilityRefreshResolve = resolve;
      this.capabilityRefreshReject = reject;

      setTimeout(() => {
        if (this.capabilityRefreshPromise) {
          this.rejectCapabilityRefreshRequest(new Error("capability refresh timed out"));
        }
      }, this.capabilityRefreshTimeoutMs);

      this.getDashboardData();
    });
    return this.capabilityRefreshPromise;
  };

  scheduleCapabilityRefresh = () => {
    if (this.capabilityRefreshTimer) {
      clearTimeout(this.capabilityRefreshTimer);
      this.capabilityRefreshTimer = null;
    }
    if (this.capabilityBySpaceUUID.size === 0) {
      return;
    }

    let earliestExpiryMs = Number.POSITIVE_INFINITY;
    this.capabilityBySpaceUUID.forEach((capability) => {
      const expiresAtMs = Number(capability.expires_at || 0) * 1000;
      if (Number.isFinite(expiresAtMs) && expiresAtMs > 0 && expiresAtMs < earliestExpiryMs) {
        earliestExpiryMs = expiresAtMs;
      }
    });
    if (!Number.isFinite(earliestExpiryMs)) {
      return;
    }

    const delay = Math.max(1000, earliestExpiryMs - Date.now() - this.capabilitySkewMs);
    this.capabilityRefreshTimer = setTimeout(() => {
      this.requestCapabilityRefresh().catch((err) => {
        console.error("capability refresh failed:", err);
      });
    }, delay);
  };

  sendWithCapability = (spaceUUID, sendFn) => {
    const execute = (token, allowRetry) => {
      sendFn(token || "");
      if (spaceUUID && allowRetry) {
        this.lastCapabilityAction = {
          attemptedAt: Date.now(),
          retried: false,
          spaceUUID,
          sendFn,
        };
      } else {
        this.lastCapabilityAction = null;
      }
    };

    if (!spaceUUID) {
      execute(this.getCapabilityToken(spaceUUID), false);
      return;
    }

    if (this.hasFreshCapability(spaceUUID)) {
      execute(this.getCapabilityToken(spaceUUID), true);
      return;
    }

    this.requestCapabilityRefresh()
      .then(() => {
        const refreshedToken = this.getCapabilityToken(spaceUUID);
        if (!refreshedToken) {
          platform.alert("Session permissions expired. Please reopen the space.");
          return;
        }
        execute(refreshedToken, true);
      })
      .catch((err) => {
        console.error("capability refresh failed:", err);
        platform.alert("Failed to refresh permissions. Please retry.");
      });
  };

  retryCapabilityActionFromError = (errorMessage) => {
    const message = String(errorMessage || "").toLowerCase();
    if (!message.includes("unauthorized")) {
      return false;
    }
    const action = this.lastCapabilityAction;
    if (!action || action.retried) {
      return false;
    }
    if (Date.now() - action.attemptedAt > this.capabilityRetryWindowMs) {
      this.lastCapabilityAction = null;
      return false;
    }

    action.retried = true;
    this.requestCapabilityRefresh()
      .then(() => {
        const refreshedToken = this.getCapabilityToken(action.spaceUUID);
        if (!refreshedToken) {
          platform.alert("Session permissions expired. Please reopen the space.");
          this.lastCapabilityAction = null;
          return;
        }
        action.sendFn(refreshedToken);
      })
      .catch((err) => {
        console.error("capability retry refresh failed:", err);
        platform.alert(errorMessage || "Unauthorized");
      });
    return true;
  };

  joinAllSpaces = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      const uuids = data.data.spaces.map((space) => space.uuid);
      const capabilityTokens = {};
      uuids.forEach((spaceUUID) => {
        const token = this.getCapabilityToken(spaceUUID);
        if (token) {
          capabilityTokens[spaceUUID] = token;
        }
      });
      this.socket.send(
        JSON.stringify({
          type: "join_all_spaces",
          data: {
            space_uuids: uuids,
            capability_tokens: capabilityTokens,
          },
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
      const spaceUUID = data?.uuid || null;
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { ...data };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "delete_space", data: payload }));
      });
    }
  };

  createChannel = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      const spaceUUID = data?.space_uuid || null;
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { ...data };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "create_channel", data: payload }));
      });
    }
  };

  deleteChannel = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      const spaceUUID = data?.space_uuid || null;
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { ...data };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "delete_channel", data: payload }));
      });
    }
  };

  inviteUser = (data) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      const spaceUUID = data?.space_uuid || null;
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { ...data };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "invite_user", data: payload }));
      });
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
      const spaceUUID = data?.space_uuid || null;
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { ...data };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "remove_space_user", data: payload }));
      });
    }
  };

  joinChannel = (spaceUUIDOrChannelUUID, maybeChannelUUID = null) => {
    const channelUUID = maybeChannelUUID || spaceUUIDOrChannelUUID;
    const spaceUUID = maybeChannelUUID ? spaceUUIDOrChannelUUID : null;
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { uuid: channelUUID };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "join_channel", data: payload }));
      });
    }
  };

  sendMessage = (data, spaceUUID = null) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      const resolvedSpaceUUID = spaceUUID || data?.envelope?.space_uuid || null;
      this.sendWithCapability(resolvedSpaceUUID, (capabilityToken) => {
        const payload = { ...data };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(JSON.stringify({ type: "chat", data: payload }));
      });
    }
  };

  getMessages = (timestamp, spaceUUID = null) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.sendWithCapability(spaceUUID, (capabilityToken) => {
        const payload = { before_unix_time: timestamp };
        if (capabilityToken) {
          payload.capability_token = capabilityToken;
        }
        this.socket.send(
          JSON.stringify({
            type: "get_messages",
            data: payload,
          })
        );
      });
    }
  };

  hardClose = () => {
    this.manualClose = true;
    this.close();
  };

  close = () => {
    if (this.capabilityRefreshTimer) {
      clearTimeout(this.capabilityRefreshTimer);
      this.capabilityRefreshTimer = null;
    }
    this.rejectCapabilityRefreshRequest(new Error("socket closing"));
    if (this.socket) {
      this.socket.close();
    }
  };
}
