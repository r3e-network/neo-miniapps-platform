declare module "builtin/App" {
  export interface BuiltinAppProps {
    appId?: string;
    view?: string;
  }

  const RemoteApp: React.ComponentType<BuiltinAppProps>;
  export default RemoteApp;
  export const App: React.ComponentType<BuiltinAppProps>;
}
