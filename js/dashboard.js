import DashModal from "./components/dashModal.js";
import SidebarComponent from "./components/sidebar.js";
import MainContentComponent from "./components/mainContent.js";
import SocketConn from "./lib/socketConn.js";

class DashboardApp {
  constructor(domComponent) {
    this.domComponent = domComponent;
    this.domComponent.classList.add("dashboard-container");
    this.data = null;
    this.sidebar = null;
    this.dashModal = null;
    this.mainContent = null;
    this.currentSpaceUUID = null;
    this.socketConn = null;
  }

  async initialize() {
    // Get dash data
    try {
      const res = await fetch("/api/dashboard_data");
      this.data = await res.json();
      console.log(this.data);
    } catch (error) {
      console.log(error);
    }

    // Start socket conn
    this.socketConn = new SocketConn({
      renderChatAppMessage: this.renderChatAppMessage,
      handleAddUser: this.handleAddUser,
      handleRemoveUser: this.handleRemoveUser,
      handleNewChannel: this.handleNewChannel,
      handleDeleteChannel: this.handleDeleteChannel,
    });
    // Init other comonents
    this.dashModal = new DashModal(this);
    this.sidebar = new SidebarComponent({
      data: this.data,
      getCurrentSpaceUUID: this.getCurrentSpaceUUID,
      createNewSpace: this.createNewSpace,
      openSpaceSettings: this.openSpaceSettings,
      loadChannel: this.loadChannel,
    });
    this.mainContent = new MainContentComponent({
      socketConn: this.socketConn,
    });

    this.render();
  }

  getCurrentSpaceUUID = () => {
    return this.currentSpaceUUID;
  };

  openSpaceSettings = (space) => {
    this.currentSpaceUUID = space.UUID;
    this.dashModal.open(space);
  };

  closeSpaceSettings = () => {
    this.dashModal.close();
    this.currentSpaceUUID = null;
  };

  createNewSpace = async () => {
    const spaceName = window.prompt("Please enter Space 'Name'");
    if (!spaceName) return;
    try {
      const response = await fetch("/api/new_space", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: spaceName }),
      });
      const result = await response.json();
      if (result.Space) {
        this.data.spaces.push(result.Space);
        this.sidebar.addSpace(result.Space);
      }
    } catch (error) {
      console.log(error);
    }
  };

  inviteUser = async () => {
    if (!this.currentSpaceUUID) return;
    const invitedUserEmail = window.prompt(
      "Please enter email address of user"
    );
    if (!invitedUserEmail) return;
    try {
      const response = await fetch(
        `/api/new_space_user/${this.currentSpaceUUID}`,
        {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            userEmail: invitedUserEmail.trim(),
          }),
        }
      );
      const result = await response.json();
      if (response.ok) {
        window.alert(`Invite Sent to ${invitedUserEmail}!`);
      } else {
        window.alert(`ERROR: ${result.error}`);
        return;
      }
    } catch (error) {
      console.log(error);
    }
  };

  handleAddUser = (newUserData) => {
    const spaceToUpdate = this.data.spaces.find(
      (space) => space.UUID === newUserData.data.SpaceUUID
    );
    if (spaceToUpdate) {
      spaceToUpdate.Users.push({
        ID: newUserData.data.ID,
        Username: newUserData.data.Username,
      });
      this.sidebar.spaceUserListComponent.render();
    }
  };

  handleRemoveUser = (userData) => {
    const spaceToUpdate = this.data.spaces.find(
      (space) => space.UUID === userData.data.SpaceUUID
    );
    if (spaceToUpdate) {
      const indexOfUser = spaceToUpdate.Users.findIndex(
        (user) => user.ID === userData.data.ID
      );
      spaceToUpdate.Users.splice(indexOfUser, 1);
      this.sidebar.spaceUserListComponent.render();

      if (userData.data.ID == this.data.user.ID) { // If the current user is removed from the space
        this.data.spaces.splice(this.data.spaces.indexOf(spaceToUpdate), 1);
        this.sidebar.render();
        this.mainContent.render();
        this.dashModal.render();
      }
    }
  };

  deleteSpace = async () => {
    if (!this.currentSpaceUUID) return;
    if (
      !window.confirm(
        "Are you sure you want to delete this space? This action cannot be undone."
      )
    )
      return;
    try {
      const response = await fetch(`/api/space/${this.currentSpaceUUID}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (response.ok) {
        // Store the current channel UUID before updating the data
        const currentChannelUUID = this.mainContent.currentChannelUUID;

        // Update the data structure
        this.data.spaces = this.data.spaces.filter(
          (s) => s.UUID !== this.currentSpaceUUID
        );

        // Preserve the channelsOpen state of remaining spaces
        const remainingComponents = this.sidebar.spaceComponents.filter(
          (comp) => comp.space.UUID !== this.currentSpaceUUID
        );
        this.sidebar.spaceComponents = remainingComponents;

        // Check if the current channel still exists in any space
        const channelStillExists = this.data.spaces.some((space) =>
          space.Channels.some((channel) => channel.UUID === currentChannelUUID)
        );

        if (!channelStillExists) {
          this.mainContent.render();
        }

        // remove current space UUID and re-render sidebar and space users list
        this.currentSpaceUUID = null;
        this.sidebar.render();
        this.sidebar.spaceUserListComponent.render();
        this.closeSpaceSettings();
      } else {
        const result = await response.json();
        window.alert(`ERROR: ${result.error}`);
      }
    } catch (error) {
      console.log(error);
    }
  };

  loadChannel = (spaceUUID, channelUUID) => {
    this.currentSpaceUUID = spaceUUID;

    this.socketConn.joinChannel(channelUUID); // Join the socket to the channel
    this.sidebar.spaceUserListComponent.render(); // Update the users list
    this.mainContent.renderChannel(channelUUID);
  };

  createNewChannel = async () => {
    const currentSpaceUUID = this.getCurrentSpaceUUID();

    if (!currentSpaceUUID) return;
    const channelName = window.prompt("Please enter Channel 'Name'");
    if (!channelName) return;
    try {
      const response = await fetch("/api/new_channel", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: channelName,
          spaceUUID: currentSpaceUUID,
        }),
      });
      const result = await response.json();
      if (result.Channel) {
        // Handled by socket 

      }
    } catch (error) {
      console.log(error);
    }
  };

  deleteChannel = async (channelUUID) => {
    const currentSpace = this.data.spaces.find(
      (space) => space.UUID === this.getCurrentSpaceUUID()
    );
    if (!currentSpace) return;
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
        currentSpace.Channels = currentSpace.Channels.filter(
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

        // Re-render both the sidebar and the modal
        this.app.sidebar.render();
        this.render();
      } else {
        const result = await response.json();
        window.alert(`ERROR: ${result.error}`);
      }
    } catch (error) {
      console.log(error);
    }
  };

  handleNewChannel = (data) => {
    const spaceToUpdate = this.data.spaces.find(
      (space) => space.UUID === data.data.SpaceUUID
    );

    if (spaceToUpdate) {
      // Update local data
      spaceToUpdate.Channels.push(data.data);
      // render
      const spaceElemToUpdate = this.sidebar.spaceComponents.find(
        (elem) => elem.space.UUID == data.data.SpaceUUID
      );
      // Update the channel data on the elem 
      spaceElemToUpdate.render();
      // Update modal
      this.dashModal.render();
    }
  }

  handleDeleteChannel = (data) => {
    const spaceToUpdate = this.data.spaces.find(
      (space) => space.UUID === data.data.SpaceUUID
    );

    if (spaceToUpdate) {
      // Update local data
      const index = spaceToUpdate.Channels.findIndex(channel => channel.ID === data.data.ID);
      spaceToUpdate.Channels.splice(index, 1);
      // render
      const spaceElemToUpdate = this.sidebar.spaceComponents.find(
        (elem) => elem.space.UUID == data.data.SpaceUUID
      );
      // Update the channel data on the elem 
      spaceElemToUpdate.render();
      // Update modal
      this.dashModal.render();
    }
  }

  renderChatAppMessage = (data) => {
    if (this.mainContent.chatApp) {
      this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent.appendNewMessage(
        data.data
      );
      // Handle scrolling
      if (data.data.Username === this.data.user.Username) {
        this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent.scrollDown();
      } else {
        if(this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent.isScrolledToBottom()) {
          this.mainContent.chatApp.chatBoxComponent.chatBoxMessagesComponent.scrollDown();
        }
      }
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

// Initialize the dashboard app

document.addEventListener("DOMContentLoaded", () => {
  const dashboardRoot = document.getElementById("dashboard-app");
  const app = new DashboardApp(dashboardRoot);
  app.initialize();
});
