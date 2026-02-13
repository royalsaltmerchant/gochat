const relayBaseURL = import.meta.env.VITE_RELAY_BASE_URL || "https://parchchat.com";
const relayBaseURLWS = import.meta.env.VITE_RELAY_BASE_URL_WS || "wss://parchchat.com/ws";
const sfuBaseURLWS = import.meta.env.VITE_SFU_BASE_URL_WS || "wss://sfu.parchchat.com/ws";

export { relayBaseURL, relayBaseURLWS, sfuBaseURLWS };
