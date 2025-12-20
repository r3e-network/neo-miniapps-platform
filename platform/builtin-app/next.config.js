const { NextFederationPlugin } = require("@module-federation/nextjs-mf");

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  webpack(config) {
    config.plugins.push(
      new NextFederationPlugin({
        name: "builtin",
        filename: "static/chunks/remoteEntry.js",
        exposes: {
          "./App": "./src/components/BuiltinApp",
        },
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
