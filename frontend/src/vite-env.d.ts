/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_SUPABASE_URL: string;
  readonly VITE_SUPABASE_ANON_KEY: string;
  readonly VITE_NEO_RPC_URL: string;
  readonly VITE_NEO_NETWORK: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
