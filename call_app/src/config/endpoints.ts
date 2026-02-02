// Determine if we're in development or production

// const isDev = import.meta.env.DEV;
const isDev = false; // Temporary override for testing purposes

// Development endpoints
const devConfig = {
  relayHost: "localhost:8000",
  relayBaseURL: "http://localhost:8000",
  relayBaseURLWS: "ws://localhost:8000/ws",
  sfuHost: "localhost:7000",
  sfuBaseURLWS: "ws://localhost:7000/ws",
};

// Production endpoints
const prodConfig = {
  relayHost: "parchchat.com",
  relayBaseURL: "https://parchchat.com",
  relayBaseURLWS: "wss://parchchat.com/ws",
  sfuHost: "sfu.parchchat.com",
  sfuBaseURLWS: "wss://sfu.parchchat.com/ws",
};

const config = isDev ? devConfig : prodConfig;

export const { relayHost, relayBaseURL, relayBaseURLWS, sfuHost, sfuBaseURLWS } = config;
