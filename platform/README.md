# Platform Layer (Host + Built-ins + SDK + Supabase)

This folder contains the **web host** (Next.js on Vercel), the **built-in Module Federation remote**, the **MiniApp SDK**, and the **Supabase Edge/RLS** components.

- `host-app/`: Next.js host application (iframe + Module Federation loader)
- `builtin-app/`: built-in MiniApps served via Module Federation
- `sdk/`: `window.MiniAppSDK` implementation
- `edge/`: Supabase Edge functions (auth/limits/routing)
- `rls/`: Supabase RLS SQL policies
