import React from "react";
import { MiniAppInfo, MiniAppStats } from "./types";
import { colors } from "./styles";

type Props = {
  app: MiniAppInfo;
  stats?: MiniAppStats;
  onClick: () => void;
};

export function MiniAppCard({ app, stats, onClick }: Props) {
  return (
    <div onClick={onClick} style={cardStyle}>
      <div style={iconStyle}>{app.icon}</div>
      <div style={contentStyle}>
        <h3 style={titleStyle}>{app.name}</h3>
        <p style={descStyle}>{app.description}</p>
        {stats && (
          <div style={statsRow}>
            <span>📊 {stats.total_transactions} txs</span>
            <span>👥 {stats.total_users} users</span>
          </div>
        )}
      </div>
      <span style={categoryBadge}>{app.category}</span>
    </div>
  );
}

const cardStyle: React.CSSProperties = {
  background: colors.bgCard,
  borderRadius: 16,
  padding: 20,
  cursor: "pointer",
  border: `1px solid ${colors.border}`,
  transition: "all 0.2s",
  position: "relative",
};

const iconStyle: React.CSSProperties = {
  fontSize: 40,
  marginBottom: 12,
};

const contentStyle: React.CSSProperties = {
  flex: 1,
};

const titleStyle: React.CSSProperties = {
  fontSize: 18,
  fontWeight: 600,
  margin: "0 0 8px 0",
  color: colors.text,
};

const descStyle: React.CSSProperties = {
  fontSize: 14,
  color: colors.textMuted,
  margin: 0,
  lineHeight: 1.5,
};

const statsRow: React.CSSProperties = {
  display: "flex",
  gap: 16,
  marginTop: 12,
  fontSize: 12,
  color: colors.primary,
};

const categoryBadge: React.CSSProperties = {
  position: "absolute",
  top: 12,
  right: 12,
  fontSize: 11,
  padding: "4px 8px",
  borderRadius: 6,
  background: "rgba(0,212,170,0.15)",
  color: colors.primary,
  textTransform: "uppercase",
};
