/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_ORIGIN?: string;
  readonly VITE_API_BASE_URL?: string;
  readonly VITE_PROXY_TARGET?: string;
  readonly VITE_STREAM_PATH?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
