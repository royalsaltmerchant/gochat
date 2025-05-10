class SocketConn {
  constructor(props) {
    this.socket = null;
    this.renderChatAppMessage = props.renderChatAppMessage;
    this.handleAddUser = props.handleAddUser;
    this.handleRemoveUser = props.handleRemoveUser;
    this.handleNewChannel = props.handleNewChannel;
    this.handleDeleteChannel = props.handleDeleteChannel;

    this.connect();
  }

  connect = () => {
    const url = `ws://${location.host}/ws`;
    console.log('Connecting to WebSocket:', url);
    this.socket = new WebSocket(url);

    this.socket.onopen = () => {
      console.log('WebSocket connection established');
    };

    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.socket.onclose = () => {
      console.log('WebSocket connection closed');
    };

    this.socket.onmessage = (event) => {
      console.log('Received message:', event.data);
      const data = JSON.parse(event.data);
      switch (data.type) {
        case "join":
          console.log('Join message:', data);
          break;
        case "leave":
          console.log('Leave message:', data);
          break;
        case "chat":
          console.log('Chat message:', data);
          this.renderChatAppMessage(data);
          break;
        case "new-user":
          console.log('New User message', data)
          this.handleAddUser(data);
          break;
        case "remove-user":
          this.handleRemoveUser(data);
          break;
        case "new-channel":
          this.handleNewChannel(data);
          break;
        case "delete-channel":
          this.handleDeleteChannel(data);
          break;
        default:
          console.warn("Unknown message type", data);
      }


    };
  };

  sendMessage = (text) => {
    console.log('Attempting to send message:', text);
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log('Socket is open, sending message');
      // We need more than just text
      const wsMessage = {
        type: "chat",
        data: text
      }
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error('Socket is not open. State:', this.socket?.readyState);
    }
  };

  joinChannel = (channelUUID) => {
    console.log('Attempting to join channel:', channelUUID);
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log('Socket is open, sending message');
      // We need more than just text
      const wsMessage = {
        type: "join-channel",
        data: channelUUID
      }
      this.socket.send(JSON.stringify(wsMessage));
    } else {
      console.error('Socket is not open. State:', this.socket?.readyState);
    }
  };

  // leaveChannel = (channelUUID) => {
  //   console.log('Attempting to leave channel:', channelUUID);
  //   if (this.socket?.readyState === WebSocket.OPEN) {
  //     console.log('Socket is open, sending message');
  //     // We need more than just text
  //     const wsMessage = {
  //       type: "leave",
  //       data: channelUUID
  //     }
  //     this.socket.send(JSON.stringify(wsMessage));
  //   } else {
  //     console.error('Socket is not open. State:', this.socket?.readyState);
  //   }
  // };

  close = () => {
    if (this.socket) {
      console.log('Closing WebSocket connection');
      this.socket.close();
    }
  };
}

export default SocketConn;
