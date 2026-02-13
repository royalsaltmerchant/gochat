import { defineConfig } from "vite";

export default defineConfig(({ mode }) => {
  const isDesktop = mode !== "web";

  return {
    base: isDesktop ? "./" : "/client/",
    build: {
      outDir: isDesktop ? "dist" : "../../relay_server/static/client",
      emptyOutDir: true,
    },
  };
});
