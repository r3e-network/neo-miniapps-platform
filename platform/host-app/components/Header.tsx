import React from "react";
import { WalletState } from "./types";
import { colors } from "./styles";

type Props = {
  wallet: WalletState;
  onConnect: () => void;
};

export function Header({ wallet, onConnect }: Props) {
  return (
    <header style={headerStyle}>
      <div style={logoStyle}>
        <div style={logoIcon}>N</div>
        <span>Neo MiniApps</span>
      </div>
      <button onClick={onConnect} style={walletBtn}>
        {wallet.connected ? `${wallet.address.slice(0, 6)}...${wallet.address.slice(-4)}` : "Connect Wallet"}
      </button>
    </header>
  );
}

const headerStyle: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  alignItems: "center",
  padding: "16px 24px",
  borderBottom: `1px solid ${colors.border}`,
  background: "rgba(5,8,16,0.9)",
  backdropFilter: "blur(20px)",
  position: "sticky",
  top: 0,
  zIndex: 100,
};

const logoStyle: React.CSSProperties = {
  display: "flex",
  alignItems: "center",
  gap: 10,
  fontSize: 20,
  fontWeight: 700,
  color: colors.text,
};

const logoIcon: React.CSSProperties = {
  width: 36,
  height: 36,
  borderRadius: 10,
  background: `linear-gradient(135deg, ${colors.primary}, ${colors.primaryDark})`,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  fontWeight: 800,
  color: "#fff",
};

const walletBtn: React.CSSProperties = {
  padding: "10px 20px",
  borderRadius: 10,
  border: "none",
  background: colors.primary,
  color: "#000",
  fontWeight: 600,
  cursor: "pointer",
};
