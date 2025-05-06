import createElement from "./components/createElement.js";
import ChatApp from './chatapp.js';

class DashboardApp {
  constructor(domComponent) {
    this.domComponent = domComponent;
    this.domComponent.classList.add('dashboard-container');
    this.data = null;
    this.sidebar = null;
    this.mainContent = null;
    this.spaceSettingsModal = null;
    this.currentSpaceUUID = null;
  }

  async initialize() {
    const res = await fetch('/api/dashboard_data');
    this.data = await res.json();
    this.render();
  }

  render() {
    this.domComponent.innerHTML = "";
    this.sidebar = new SidebarComponent(
      this.data,
      this.createNewSpace,
      this.handleAcceptInvite,
      this.handleDeclineInvite,
      this.openSpaceSettings,
      this.loadChannel
    );
    this.mainContent = new MainContentComponent();
    this.spaceSettingsModal = new SpaceSettingsModal(this);
    this.domComponent.append(
      this.sidebar.domComponent,
      this.mainContent.domComponent,
      this.spaceSettingsModal.domComponent
    );
  }

  openSpaceSettings = (space) => {
    this.currentSpaceUUID = space.UUID;
    this.spaceSettingsModal.open(space);
  }

  closeSpaceSettings = () => {
    this.spaceSettingsModal.close();
    this.currentSpaceUUID = null;
  }

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
      if (result.space) {
        // Initialize the Channels array with the initial channel
        result.space.Channels = [result.channel];
        this.data.spaces.push(result.space);
        this.sidebar.addSpace(result.space);
      }
    } catch (error) {
      console.log(error);
    }
  }

  inviteUser = async () => {
    if (!this.currentSpaceUUID) return;
    const invitedUserEmail = window.prompt("Please enter email address of user");
    if (!invitedUserEmail) return;
    try {
      const response = await fetch("/api/new_space_user", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ userEmail: invitedUserEmail.trim(), spaceUUID: this.currentSpaceUUID }),
      });
      const result = await response.json();
      if (response.status === 400) {
        window.alert(`ERROR: ${result.error}`);
        return;
      }
      window.alert(`Invite Sent to ${invitedUserEmail}!`);
    } catch (error) {
      console.log(error);
    }
  }

  deleteSpace = async () => {
    if (!this.currentSpaceUUID) return;
    if (!window.confirm("Are you sure you want to delete this space? This action cannot be undone.")) return;
    try {
      const response = await fetch(`/api/space/${this.currentSpaceUUID}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (response.ok) {
        // Store the current channel UUID before updating the data
        const currentChannelUUID = this.mainContent.currentChannelUUID;
        
        // Update the data structure
        this.data.spaces = this.data.spaces.filter(s => s.UUID !== this.currentSpaceUUID);
        
        // Preserve the channelsOpen state of remaining spaces
        const remainingComponents = this.sidebar.spaceComponents.filter(comp => comp.space.UUID !== this.currentSpaceUUID);
        this.sidebar.spaceComponents = remainingComponents;
        
        // Check if the current channel still exists in any space
        const channelStillExists = this.data.spaces.some(space => 
          space.Channels.some(channel => channel.UUID === currentChannelUUID)
        );
        
        if (!channelStillExists) {
          this.mainContent.render();
        }
        
        this.sidebar.render();
        this.closeSpaceSettings();
      } else {
        const result = await response.json();
        window.alert(`ERROR: ${result.error}`);
      }
    } catch (error) {
      console.log(error);
    }
  }

  handleAcceptInvite = async (inviteID) => {
    try {
      const response = await fetch("/api/accept_invite", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ spaceUserID: inviteID }),
      });
      const result = await response.json();
      if (result.space) {
        this.data.spaces.push(result.space);
        this.sidebar.addSpace(result.space);
        this.data.invites = this.data.invites.filter(i => i.ID !== inviteID);
        this.sidebar.render();
      }
    } catch (error) {
      console.log(error);
    }
  }

  handleDeclineInvite = async (inviteID) => {
    try {
      const response = await fetch("/api/decline_invite", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ spaceUserID: inviteID }),
      });
      if (response.status === 400) {
        window.alert("Error: Failed to decline invite!");
        return;
      }
      this.data.invites = this.data.invites.filter(i => i.ID !== inviteID);
      this.sidebar.render();
    } catch (error) {
      console.log(error);
    }
  }

  loadChannel = (channelUUID) => {
    this.mainContent.renderChannel(channelUUID);
  }

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
          spaceUUID: this.currentSpace.UUID 
        }),
      });
      const result = await response.json();
      if (result.channel) {
        this.currentSpace.Channels.push(result.channel);
        this.app.sidebar.render();
        this.render();
      }
    } catch (error) {
      console.log(error);
    }
  }

  deleteChannel = async (channelUUID) => {
    if (!this.currentSpace) return;
    if (!window.confirm("Are you sure you want to delete this channel? This action cannot be undone.")) return;
    try {
      const response = await fetch(`/api/channel/${channelUUID}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (response.ok) {
        // Update the channels array - this will automatically update the app's data structure
        this.currentSpace.Channels = this.currentSpace.Channels.filter(c => c.UUID !== channelUUID);
        
        // Check if the current channel still exists in any space
        const channelStillExists = this.app.data.spaces.some(space => 
          space.Channels.some(channel => channel.UUID === this.app.mainContent.currentChannelUUID)
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
  }
}

class SidebarComponent {
  constructor(data, onCreateSpace, onAcceptInvite, onDeclineInvite, onOpenSpaceSettings, onLoadChannel) {
    this.data = data;
    this.spaceComponents = [];
    this.domComponent = createElement('div', { class: 'sidebar' });
    this.onCreateSpace = onCreateSpace;
    this.onAcceptInvite = onAcceptInvite;
    this.onDeclineInvite = onDeclineInvite;
    this.onOpenSpaceSettings = onOpenSpaceSettings;
    this.onLoadChannel = onLoadChannel;
    this.initSpaceComponents();
    this.render();
  }

  initSpaceComponents = () => {
    this.spaceComponents = this.data.spaces.map(space =>
      new SpaceItemComponent(space, this.data.user, this.onOpenSpaceSettings, this.onLoadChannel)
    );
  }

  addSpace = (space) => {
    const comp = new SpaceItemComponent(space, this.data.user, this.onOpenSpaceSettings, this.onLoadChannel);
    this.spaceComponents.push(comp);
    this.render();
  }

  render = () => {
    this.domComponent.innerHTML = "";
    // Invites
    if (this.data.invites && this.data.invites.length > 0) {
      this.domComponent.append(
        createElement('div', { class: 'invites-section' }, [
          createElement('h3', {}, 'Pending Invites'),
          ...this.data.invites.map(invite =>
            createElement('div', { class: 'pending-invites-item' }, [
              createElement('span', {}, invite.Name),
              createElement(
                'button',
                { class: 'btn-small accept-invite' },
                'Accept',
                { type: 'click', event: () => this.onAcceptInvite(invite.ID) }
              ),
              createElement(
                'button',
                { class: 'btn-small btn-red decline-invite' },
                'Decline',
                { type: 'click', event: () => this.onDeclineInvite(invite.ID) }
              ),
            ])
          )
        ])
      );
    }
    // Spaces
    this.domComponent.append(
      createElement('h3', {}, 'Spaces'),
      createElement('div', { id: 'spaces-list' },
        this.spaceComponents.map(comp => comp.domComponent)
      ),
      createElement(
        'button',
        { class: 'create-space-btn', id: 'new-space-btn' },
        '+ Create New Space',
        { type: 'click', event: this.onCreateSpace }
      )
    );
  }
}

class SpaceItemComponent {
  constructor(space, user, onOpenSpaceSettings, onLoadChannel) {
    this.space = space;
    this.user = user;
    this.onOpenSpaceSettings = onOpenSpaceSettings;
    this.onLoadChannel = onLoadChannel;
    this.channelsOpen = false;
    this.domComponent = createElement('div', { class: 'space-item' });
    this.render();
  }

  renderActions = (isAuthor) => (
    isAuthor
      ? [createElement('button', { class: 'btn-icon open-settings' }, '⚙️', { type: 'click', event: (e) => {
          e.stopPropagation(); // Prevent the click from triggering toggleChannels
          this.onOpenSpaceSettings(this.space);
        }})]
      : []
  );

  renderHeader = (isAuthor) => (
    createElement(
      'div',
      { class: 'space-header' },
      [
        createElement('span', {}, this.space.Name),
        createElement('div', { class: 'space-actions' }, this.renderActions(isAuthor))
      ],
      { type: 'click', event: this.toggleChannels }
    )
  );

  renderChannelList = () => (
    createElement(
      'div',
      { class: 'channel-list' },
      (this.space.Channels || []).map(channel =>
        createElement(
          'div',
          { class: 'channel-item' },
          channel.Name,
          { type: 'click', event: () => this.onLoadChannel(channel.UUID) }
        )
      )
    )
  );

  toggleChannels = () => {
    this.channelsOpen = !this.channelsOpen;
    this.render();
  }

  render = () => {
    // Store the current state
    const wasOpen = this.channelsOpen;
    
    // Update the space data
    this.space = this.space;
    
    // Restore the state
    this.channelsOpen = wasOpen;
    
    this.domComponent.innerHTML = "";
    const isAuthor = String(this.space.AuthorID) === String(this.user.id);
    this.domComponent.append(
      this.renderHeader(isAuthor),
      ...(this.channelsOpen ? [this.renderChannelList()] : [])
    );
  }
}

class MainContentComponent {
  constructor() {
    this.domComponent = createElement('div', { id: 'channel-content', class: 'channel-content' });
    this.chatApp = null;
    this.currentChannelUUID = null;
    this.render();
  }

  renderChannel = (channelUUID) => {
    this.currentChannelUUID = channelUUID;
    this.domComponent.innerHTML = '';
    const chatAppDiv = createElement('div', { id: 'chatapp' });
    this.domComponent.append(chatAppDiv);
    if (!this.chatApp) {
      this.chatApp = new ChatApp({ domComponent: chatAppDiv });
    } else {
      this.chatApp.domComponent = chatAppDiv;
    }
    this.chatApp.initialize(channelUUID);
  }

  cleanup = () => {
    if (this.chatApp) {
      // Clean up any WebSocket connections or event listeners
      this.chatApp.cleanup?.();
      this.chatApp = null;
    }
    this.currentChannelUUID = null;
  }

  render = () => {
    this.cleanup();
    this.domComponent.innerHTML = "";
    this.domComponent.append(
      createElement('div', { class: 'no-channel-selected' }, [
        createElement('h2', {}, 'Select a channel to start chatting')
      ])
    );
  }
}

class SpaceSettingsModal {
  constructor(app) {
    this.app = app;
    this.domComponent = createElement('div', { class: 'modal' });
    this.currentSpace = null;
    this.render();
  }

  open = (space) => {
    this.currentSpace = space;
    this.domComponent.style.display = 'block';
    this.render();
  }

  close = () => {
    this.domComponent.style.display = 'none';
    this.currentSpace = null;
  }

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
          spaceUUID: this.currentSpace.UUID 
        }),
      });
      const result = await response.json();
      if (result.channel) {
        this.currentSpace.Channels.push(result.channel);
        // Update the space component directly instead of re-rendering the whole sidebar
        const spaceComponent = this.app.sidebar.spaceComponents.find(comp => comp.space.UUID === this.currentSpace.UUID);
        if (spaceComponent) {
          spaceComponent.render();
        }
        this.render();
      }
    } catch (error) {
      console.log(error);
    }
  }

  deleteChannel = async (channelUUID) => {
    if (!this.currentSpace) return;
    if (!window.confirm("Are you sure you want to delete this channel? This action cannot be undone.")) return;
    try {
      const response = await fetch(`/api/channel/${channelUUID}`, {
        method: "DELETE",
        credentials: "include",
      });
      if (response.ok) {
        // Update the channels array - this will automatically update the app's data structure
        this.currentSpace.Channels = this.currentSpace.Channels.filter(c => c.UUID !== channelUUID);
        
        // Check if the current channel still exists in any space
        const channelStillExists = this.app.data.spaces.some(space => 
          space.Channels.some(channel => channel.UUID === this.app.mainContent.currentChannelUUID)
        );
        
        if (!channelStillExists) {
          this.app.mainContent.render();
        }
        
        // Update the space component directly instead of re-rendering the whole sidebar
        const spaceComponent = this.app.sidebar.spaceComponents.find(comp => comp.space.UUID === this.currentSpace.UUID);
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
  }

  render = () => {
    if (!this.currentSpace) {
      this.domComponent.innerHTML = "";
      return;
    }

    const isAuthor = this.currentSpace.AuthorID === this.app.data.user.id;

    this.domComponent.innerHTML = "";
    this.domComponent.append(
      createElement('div', { class: 'modal-content' }, [
        createElement('div', { class: 'modal-header' }, [
          createElement('h2', {}, `Space Settings: ${this.currentSpace.Name}`),
          createElement('button', { class: 'close-btn' }, '×', { type: 'click', event: this.close })
        ]),
        createElement('div', { class: 'modal-body' }, [
          // Space Management Section
          createElement('div', { class: 'settings-section' }, [
            createElement('h3', {}, 'Space Management'),
            createElement('div', { class: 'settings-actions' }, [
              createElement('button', { class: 'btn' }, 'Invite User', { type: 'click', event: this.app.inviteUser }),
              isAuthor && createElement('button', { class: 'btn btn-red' }, 'Delete Space', { type: 'click', event: this.app.deleteSpace })
            ])
          ]),
          // Channel Management Section
          createElement('div', { class: 'settings-section' }, [
            createElement('h3', {}, 'Channel Management'),
            createElement('div', { class: 'settings-actions' }, [
              createElement('button', { class: 'btn' }, '+ Create Channel', { type: 'click', event: this.createNewChannel })
            ]),
            createElement('div', { class: 'channels-list' }, 
              this.currentSpace.Channels.map(channel => 
                createElement('div', { class: 'channel-item' }, [
                  createElement('span', {}, channel.Name),
                  createElement('button', { class: 'btn-small btn-red' }, 'Delete', { 
                    type: 'click', 
                    event: () => this.deleteChannel(channel.UUID)
                  })
                ])
              )
            )
          ])
        ])
      ])
    );
  }
}

// Initialize the dashboard app

document.addEventListener('DOMContentLoaded', () => {
  const dashboardRoot = document.getElementById('dashboard-app');
  const app = new DashboardApp(dashboardRoot);
  app.initialize();
});