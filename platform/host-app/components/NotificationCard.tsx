import React from "react";
import { MiniAppNotification } from "./types";
import { colors } from "./styles";

type Props = {
  notification: MiniAppNotification;
};

export function NotificationCard({ notification }: Props) {
  const timeAgo = getTimeAgo(notification.created_at);

  return (
    <div style={cardStyle}>
      <div style={headerRow}>
        <span style={typeTag}>{notification.notification_type}</span>
        <span style={timeStyle}>{timeAgo}</span>
      </div>
      <h4 style={titleStyle}>{notification.title}</h4>
      <p style={contentStyle}>{notification.content}</p>
    </div>
  );
}

function getTimeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

const cardStyle: React.CSSProperties = {
  background: colors.bgCard,
  borderRadius: 12,
  padding: 16,
  border: `1px solid ${colors.border}`,
};

const headerRow: React.CSSProperties = {
  display: "flex",
  justifyContent: "space-between",
  marginBottom: 8,
};

const typeTag: React.CSSProperties = {
  fontSize: 11,
  padding: "2px 6px",
  borderRadius: 4,
  background: "rgba(52,152,219,0.2)",
  color: colors.accent,
};

const timeStyle: React.CSSProperties = {
  fontSize: 12,
  color: colors.textMuted,
};

const titleStyle: React.CSSProperties = {
  fontSize: 14,
  fontWeight: 600,
  margin: "0 0 4px 0",
  color: colors.text,
};

const contentStyle: React.CSSProperties = {
  fontSize: 13,
  color: colors.textMuted,
  margin: 0,
  lineHeight: 1.4,
};
