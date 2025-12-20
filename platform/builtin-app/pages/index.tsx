import Head from "next/head";
import { useRouter } from "next/router";

import BuiltinApp from "../src/components/BuiltinApp";

export default function BuiltinHostPage() {
  const router = useRouter();
  const appId = typeof router.query.app === "string" ? router.query.app : undefined;
  const view = typeof router.query.view === "string" ? router.query.view : undefined;

  return (
    <>
      <Head>
        <title>Neo Built-in MiniApps</title>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
      </Head>
      <BuiltinApp appId={appId} view={view} />
    </>
  );
}
