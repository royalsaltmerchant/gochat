const wailsPlatform = {
  greet(name) {
    return window.go.main.App.Greet(name);
  },

  alert(message) {
    return window.go.main.App.Alert(message);
  },

  confirm(message) {
    return window.go.main.App.Confirm(message);
  },

  openExternal(url) {
    return window.go.main.App.OpenInBrowser(url);
  },

  saveAuthToken(token) {
    return window.go.main.App.SaveAuthToken(token);
  },

  loadAuthToken() {
    return window.go.main.App.LoadAuthToken();
  },

  removeAuthToken() {
    return window.go.main.App.RemoveAuthToken();
  },

  getHosts() {
    return window.go.main.App.GetHosts();
  },

  verifyHostKey(hostUUID) {
    return window.go.main.App.VerifyHostKey(hostUUID);
  },

  removeHost(hostUUID) {
    return window.go.main.App.RemoveHost(hostUUID);
  },
};

export default wailsPlatform;
