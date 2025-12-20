import Head from "next/head";
import dynamic from "next/dynamic";
import { useRouter } from "next/router";
import { Component, ReactNode } from "react";

const BuiltinMiniApp = dynamic(
  () => import("builtin/App").then((mod: any) => mod.default ?? mod.App),
  {
    ssr: false,
    loading: () => <p>Loading federated MiniApp…</p>,
  },
);

class RemoteErrorBoundary extends Component<{ children: ReactNode }, { error?: Error }> {
  state: { error?: Error } = {};

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    if (!this.state.error) return this.props.children;

    return (
      <div style={{ padding: 12, border: "1px solid #f2c6c6", borderRadius: 8, background: "#fff6f6" }}>
        <div style={{ fontWeight: 600, marginBottom: 6 }}>Failed to load federated MiniApp</div>
        <div style={{ fontSize: 12, color: "#8a2c2c" }}>{this.state.error.message}</div>
      </div>
    );
  }
}

export default function FederatedMiniApp() {
  const router = useRouter();
  const appId = typeof router.query.app === "string" ? router.query.app : undefined;
  const view = typeof router.query.view === "string" ? router.query.view : undefined;
  const remotes = process.env.NEXT_PUBLIC_MF_REMOTES || "";

  return (
    <>
      <Head>
        <title>Federated MiniApp Host</title>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>
      <main style={{ padding: 24, fontFamily: "system-ui, sans-serif", maxWidth: 960 }}>
        <h1 style={{ margin: "0 0 12px" }}>Federated MiniApp Host</h1>
        <p style={{ margin: "0 0 12px", fontSize: 14 }}>
          Built-in MiniApps can be served as Module Federation remotes. This page loads the{" "}
          <code>builtin/App</code> module from the configured remote and passes optional{" "}
          <code>?app=&lt;app_id&gt;</code> parameters.
        </p>
        <div style={{ marginBottom: 12, fontSize: 12 }}>
          <div>
            <strong>Expected remote:</strong> <code>builtin</code> exposing <code>./App</code>
          </div>
          <div>
            <strong>NEXT_PUBLIC_MF_REMOTES:</strong>{" "}
            <code>{remotes || "not set"}</code>
          </div>
          <div>
            Example:{" "}
            <code>
              NEXT_PUBLIC_MF_REMOTES=builtin@https://cdn.miniapps.com/miniapps/builtin-mf
            </code>
          </div>
        </div>
        <RemoteErrorBoundary>
          <BuiltinMiniApp appId={appId} view={view} />
        </RemoteErrorBoundary>
      </main>
    </>
  );
}
