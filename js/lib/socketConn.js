class SocketConn {
  constructor(props) {
    this.roomUUID = props.roomUUID;
    this.socket = null;
    this.chatapp = props.chatapp;

    this.connect();
  }

  connect = () => {
    const authToken = localStorage.getItem("authToken");
    const url = `ws://${location.host}/ws/${encodeURIComponent(
      this.roomUUID
    )}?auth=${encodeURIComponent(authToken)}`;
    this.socket = new WebSocket(url);

    this.socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      switch (data.type) {
        case "join":
          console.log(data);
          break;
        case "leave":
          console.log(data);
          break;
        case "chat":
          console.log(data);
          break;
        default:
          console.warn("Unknown message type", data);
      }

      this.chatapp.chatBoxComponent.chatBoxMessagesComponent.chatBoxMessages.push(
        data.data
      );
      this.chatapp.chatBoxComponent.chatBoxMessagesComponent.render()
    };
  };

  sendMessage = (text) => {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(text);
    }
  };
}

export default SocketConn;
