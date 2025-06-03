// dev
// const relayHost = "99.36.161.96:8000"
// const relayBaseURL = "http://" + relayHost;
// const relayBaseURLWS = "ws://" + relayHost + "/ws";

// prod
const relayHost = "parchchat.com";
const relayBaseURL = "https://" + relayHost;
const relayBaseURLWS = "wss://" + relayHost + "/ws";

const sfuHost = "sfu.parchchat.com";

const sfuBaseURLWS = "wss://" + sfuHost + "/ws";

export { relayBaseURL, relayBaseURLWS, sfuBaseURLWS };
