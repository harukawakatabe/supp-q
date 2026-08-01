import { defineConfig, loadEnv } from "vite";
import uni from "@dcloudio/vite-plugin-uni";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  return {
    plugins: [uni()],
    server: {
      host: "127.0.0.1",
      port: 5173,
      strictPort: true,
      proxy: env.VITE_API_BASE_URL
        ? undefined
        : {
            "/api": {
              target: "http://127.0.0.1:8080",
              changeOrigin: true,
            },
          },
    },
  };
});
