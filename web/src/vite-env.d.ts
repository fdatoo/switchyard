/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_SY_VERSION?: string;
  readonly VITE_SY_BUILD_SHA?: string;
  readonly VITE_SY_BUILD_DATE?: string;
  readonly VITE_SY_LICENSE?: string;
  readonly VITE_SY_BINARY_FINGERPRINT?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
