class SocketConn {
  constructor(props) {
    this.channelUUID = props.channelUUID;
    this.socket = null;
    this.chatapp = props.chatapp;

    this.connect();
  }

  connect = () => {
    const url = `ws://${location.host}/ws/${encodeURIComponent(this.channelUUID)}`;
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
          break;
        default:
          console.warn("Unknown message type", data);
      }

      this.chatapp.chatBoxComponent.chatBoxMessagesComponent.chatBoxMessages.push(
        data.data
      );
      this.chatapp.chatBoxComponent.chatBoxMessagesComponent.render();
    };
  };

  sendMessage = (text) => {
    console.log('Attempting to send message:', text);
    if (this.socket?.readyState === WebSocket.OPEN) {
      console.log('Socket is open, sending message');
      this.socket.send(text);
    } else {
      console.error('Socket is not open. State:', this.socket?.readyState);
    }
  };

  close = () => {
    if (this.socket) {
      console.log('Closing WebSocket connection');
      this.socket.close();
    }
  };
}

export default SocketConn;
