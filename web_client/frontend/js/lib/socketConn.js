import voiceManager from "./voiceManager";
import { relayBaseURLWS } from "./config.js";

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
    this.handleNewChannel = props.handleNewChannel;
    this.handleCreateChannelUpdate = props.handleCreateChannelUpdate;
    this.handleDeleteChannel = props.handleDeleteChannel;
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
    this.hostStatus = false;
    this.retryCount = 0;
    this.maxRetries = 1; // try to reconnect ten times before returning to index

    this.hostUUID = localStorage.getItem("hostUUID");

    if (this.hostUUID) {
      console.log("Host Key:", this.hostUUID);
      this.connect();
    } else {
      console.log("Missing host key");
      window.go.main.App.Alert(
        "App failed to start because of missing HOST KEY"
      );

      this.returnToHostList();
    }
  }

  retryConnection = () => {
    const retryDelay = 2000; // 2 seconds

    this.retryCount += 1;
    if (this.retryCount > this.maxRetries) {
      console.log("Max WebSocket reconnection attempts exceeded.");
      window.go.main.App.Alert(
        "Unable to reconnect to host. Returning to home."
      );
      this.returnToHostList(); // or your fallback route
      return;
    }

    console.log(
      `Retrying WebSocket connection (#${this.retryCount}) in ${
        retryDelay / 1000
      } seconds...`
    );

    setTimeout(() => {
      if (this.socket?.readyState !== WebSocket.OPEN) {
        this.connect();
      }
    }, retryDelay);
  };

  connect = () => {
    const url = relayBaseURLWS; // Relay socket server remote
    console.log("Connecting to WebSocket:", url);
    this.socket = new WebSocket(url);

    this.socket.onopen = () => {
      console.log("WebSocket connection established");
      this.joinHost();
    };

    this.socket.onerror = (error) => {
      console.error("WebSocket error:", error);
    };

    this.socket.onclose = (event) => {
      console.warn("WebSocket connection closed:", event.reason || "No reason");
      this.hostStatus = false;
      if (this.manualClose) return;
      this.retryConnection();
    };

    this.socket.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        // ... your switch

        switch (data.type) {
          case "host_status_result":
            console.log("Host status result message:", data);
            if (data.data.error) {
              // TODO: Stop application from running until host status is back or have user return to index page;

              // Continue trying until success
              console.log("Retrying");
              setTimeout(() => {
                this.getHostStatus();
              }, 1000);
              break;
            }

            this.hostStatus = true;
            this.joinHost();
            break;
          case "join_ack":
            console.log("Joining Result", data);
            // try to login user with JWT
            window.go.main.App.LoadAuthToken().then((token) => {
              if (token) {
                this.loginUserByToken({ token });
              } else {
                this.openDashModal({ type: "login" });
              }
            });

            break;
          case "join_error":
            console.log("Joining Result", data);
            window.go.main.App.Alert(data.data.error);
          case "login_user_success":
            console.log("Login user success", data);
            window.go.main.App.SaveAuthToken(data.data.token);
            this.closeDashModal();
            this.getDashboardData();
            break;
          case "dash_data_payload":
            console.log("Dash data payload", data);
            this.dashboardInitialRender(data.data);
            this.joinAllSpaces(data);
            break;
          case "update_username_success":
            console.log("Update username success", data);
            this.updateAccountUsername(data, this.hostUUID);
            break;
          case "create_space_success":
            console.log("Create space success", data);
            this.handleCreateSpace(data);
            break;
          case "delete_space_success":
            console.log("Delete space success", data);
            this.handleDeleteSpace();
            break;
          case "create_channel_success":
            console.log("Create channel success", data);
            this.handleCreateChannel(data);
            break;
          case "create_channel_update":
            console.log("Create channel update", data);
            this.handleCreateChannelUpdate(data);
            break;
          case "delete_channel_success":
            console.log("delete channel success", data);
            this.handleDeleteChannel(data);
            break;
          case "delete_channel_update":
            console.log("delete_channel_update", data);
            this.handleDeleteChannelUpdate(data);
            break;
          case "invite_user_success":
            console.log("invite user success", data);
            this.handleInviteUser(data);
            break;
          case "invite_user_update":
            console.log("invite user update", data);
            this.handleAddInvite(data);
            break;
          case "accept_invite_success":
            console.log("accept invite succes", data);
            this.handleAcceptInvite(data);
            break;
          case "accept_invite_update":
            console.log("accept invite update", data);
            this.handleAcceptInviteUpdate(data);
            break;
          case "decline_invite_success":
            console.log("decline invite success", data);
            this.handleDeclineInvite(data);
            break;
          case "leave_space_success":
            console.log("leave space success", data);
            this.handleLeaveSpace();
            break;
          case "leave_space_update":
            console.log("leave space update", data);
            this.handleLeaveSpaceUpdate(data);
            break;
          case "joined_channel":
            console.log("Join message:", data);
            break;
          case "left_channel":
            console.log("Leave message:", data);
            break;
          case "chat":
            console.log("Chat message:", data);
            this.renderChatAppMessage(data);
            break;
          case "get_messages_success":
            console.log("Get messages success", data);
            this.handleIncomingMessages(data);
            break;
          case "joined_voice_channel":
            console.log("Joined voice channel", data);
            voiceManager.voiceSubscriptions = data.data.voice_subs;
            break;
          case "left_voice_channel":
            console.log("Left voice channel", data);
            voiceManager.voiceSubscriptions = data.data.voice_subs;
            break;
          case "error":
            // TODO: handle certain types of errors for login and registration
            console.error("An error message from socket:", data);
            window.go.main.App.Alert(data.data.error);
            break;
          case "author_error":
            console.error("An error message from socket:", data);
            window.go.main.App.Alert(
              "Failed to connect to Host. Host is not currently connected to the relay server."
            );
            this.returnToHostList();
            break;
          default:
            console.log("Unhandled message type", data);
        }
      } catch (err) {
        console.error("JSON parsing error:", err);
      }
    };
  };

  // ************************ TO REMOTE RELAY *****************************

  joinHost = () => {
    console.log("Attempting to join host:");
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "join_host",
        data: {
          uuid: this.hostUUID,
          id: this.hostAuthorID,
        },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  getHostStatus = () => {
    console.log("Attempting to get host status:");
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = { type: "host_status", data: { uuid: this.hostUUID } };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  registerUser = (data) => {
    console.log("Attempting to register user:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "register_user",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  loginUser = (data) => {
    console.log("Attempting to login user:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "login_user",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  loginUserByToken = (data) => {
    console.log("Attempting to login user by token:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "login_user_by_token",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  approveLoginUser = (data) => {
    console.log("Attempting to approve login user:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "login_approved",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  getDashboardData = () => {
    console.log("Attempting to get dashboard data");
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "get_dash_data",
        data: "",
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  joinAllSpaces = (data) => {
    console.log("Attempting to join all spaces");
    if (this.socket?.readyState === WebSocket.OPEN) {
      const uuids = data.data.spaces.map((space) => {
        return space.uuid;
      });
      const wsMessage = {
        type: "join_all_spaces",
        data: {
          space_uuids: uuids,
        },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  updateUsername = (data) => {
    console.log("Attempting to update username:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "update_username",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  createSpace = (data) => {
    console.log("Attempting to create new space:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "create_space",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  deleteSpace = (data) => {
    console.log("Attempting to delete space:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "delete_space",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  createChannel = (data) => {
    console.log("Attempting to create new channel:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "create_channel",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  deleteChannel = (data) => {
    console.log("Attempting to delete channel:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "delete_channel",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  inviteUser = (data) => {
    console.log("Attempting to invite user:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "invite_user",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  acceptInvite = (data) => {
    console.log("Attempting to accept invite:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "accept_invite",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  declineInvite = (data) => {
    console.log("Attempting to decline invite:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "decline_invite",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  leaveSpace = (data) => {
    console.log("Attempting to leave space:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "leave_space",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  removeSpaceUser = (data) => {
    console.log("Attempting to remove space user:", data);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "remove_space_user",
        data: data,
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  joinChannel = (channelUUID) => {
    console.log("Attempting to join channel:", channelUUID);
    if (this.socket?.readyState === WebSocket.OPEN) {
      const wsMessage = {
        type: "join_channel",
        data: { uuid: channelUUID },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  sendMessage = (text) => {
    console.log("Attempting to send message:", text);
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log("Socket is open, sending message");
      const wsMessage = {
        type: "chat",
        data: { content: text },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  getMessages = (timestamp) => {
    console.log("Attempting to get messages:");
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log("Socket is open, sending message");
      const wsMessage = {
        type: "get_messages",
        data: {
          before_unix_time: timestamp,
        },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  channelAllowVoice = (channelUUID, allow) => {
    console.log("Attempting to allow voice for channel:");
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log("Socket is open, sending message");
      const wsMessage = {
        type: "channel_allow_voice",
        data: {
          uuid: channelUUID,
          allow: allow,
        },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  joinVoiceChannel = (streamID) => {
    console.log("Attempting to join voice channel:");
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log("Socket is open, sending message");
      const wsMessage = {
        type: "join_voice_channel",
        data: {
          stream_id: streamID,
        },
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  leaveVoiceChannel = () => {
    console.log("Attempting to leave voice channel:");
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log("Socket is open, sending message");
      const wsMessage = {
        type: "leave_voice_channel",
        data: "",
      };
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error("Socket is not open. State:", this.socket?.readyState);
    }
  };

  hardClose = () => {
    this.manualClose = true;
    this.close();
  };

  close = () => {
    if (this.socket) {
      console.log("Closing WebSocket connection");
      this.socket.close();
    }
  };
}
