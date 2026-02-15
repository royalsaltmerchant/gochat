import { defineConfig } from "vite";

export default defineConfig({
  base: "/client/",
  build: {
    outDir: "../../chat_relay/static/client",
    emptyOutDir: true,
  },
});
