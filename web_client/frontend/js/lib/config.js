const relayBaseURL = import.meta.env.VITE_RELAY_BASE_URL || "https://chat.parchchat.com";
const relayBaseURLWS = import.meta.env.VITE_RELAY_BASE_URL_WS || "wss://chat.parchchat.com/ws";
const sfuBaseURLWS = import.meta.env.VITE_SFU_BASE_URL_WS || "";
const officialHostUUID = "5837a5c3-5268-45e1-9ea4-ee87d959d067";
const officialHostName = "Parch Community";

export { relayBaseURL, relayBaseURLWS, sfuBaseURLWS, officialHostUUID, officialHostName };
