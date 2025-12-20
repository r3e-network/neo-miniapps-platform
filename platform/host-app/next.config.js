const { NextFederationPlugin } = require("@module-federation/nextjs-mf");

function buildFederatedRemotes(isServer) {
  const raw = String(process.env.NEXT_PUBLIC_MF_REMOTES || "").trim();
  if (!raw) return {};

  const remotes = {};
  const entries = raw.split(",").map((entry) => entry.trim()).filter(Boolean);

  for (const entry of entries) {
    const separator = entry.includes("@") ? "@" : entry.includes("=") ? "=" : null;
    if (!separator) continue;
    const [nameRaw, urlRaw] = entry.split(separator);
    const name = String(nameRaw || "").trim();
    const url = String(urlRaw || "").trim();
    if (!name || !url) continue;

    const normalizedURL = url.endsWith(".js")
      ? url
      : `${url.replace(/\/$/, "")}/_next/static/${isServer ? "ssr" : "chunks"}/remoteEntry.js`;
    remotes[name] = `${name}@${normalizedURL}`;
  }

  return remotes;
}

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "X-Frame-Options", value: "DENY" },
        ],
      },
    ];
  },
  webpack(config, options) {
    const remotes = buildFederatedRemotes(options.isServer);
    config.plugins.push(
      new NextFederationPlugin({
        name: "neo_host",
        filename: "static/chunks/remoteEntry.js",
        remotes,
        exposes: {},
        shared: {
          react: { singleton: true, requiredVersion: false },
          "react-dom": { singleton: true, requiredVersion: false },
        },
      }),
    );

    return config;
  },
};

module.exports = nextConfig;
