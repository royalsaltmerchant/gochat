const relayHost = "parchchat.com";
// const relayHost = "99.36.161.96:8000"
const sfuHost = "sfu.parchchat.com";

const relayBaseURL = "https://" + relayHost;
const relayBaseURLWS = "wss://" + relayHost + "/ws";
const sfuBaseURLWS = "wss://" + sfuHost + "/ws";

export { relayBaseURL, relayBaseURLWS, sfuBaseURLWS };
