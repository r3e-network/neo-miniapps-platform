import React, { useCallback, useEffect, useState } from "react";
import { Header, MiniAppCard, NotificationCard } from "../components";
import { MiniAppInfo, MiniAppStats, MiniAppNotification, WalletState, colors } from "../components";

// Static catalog (will be replaced by API)
const MINIAPP_CATALOG: MiniAppInfo[] = [
  {
    app_id: "builtin-lottery",
    name: "Neo Lottery",
    description: "Decentralized lottery with provably fair randomness",
    icon: "🎰",
    category: "gaming",
    entry_url: "/miniapps/builtin/lottery/index.html",
    permissions: { payments: true, randomness: true },
  },
  {
    app_id: "builtin-coin-flip",
    name: "Coin Flip",
    description: "50/50 coin flip - double your GAS",
    icon: "🪙",
    category: "gaming",
    entry_url: "/miniapps/builtin/coin-flip/index.html",
    permissions: { payments: true, randomness: true },
  },
  {
    app_id: "builtin-dice-game",
    name: "Dice Game",
    description: "Roll the dice and win up to 6x",
    icon: "🎲",
    category: "gaming",
    entry_url: "/miniapps/builtin/dice-game/index.html",
    permissions: { payments: true, randomness: true },
  },
  {
    app_id: "builtin-prediction-market",
    name: "Prediction Market",
    description: "Bet on real-world events",
    icon: "📊",
    category: "defi",
    entry_url: "/miniapps/builtin/prediction-market/index.html",
    permissions: { payments: true, datafeed: true },
  },
  {
    app_id: "builtin-price-ticker",
    name: "Price Ticker",
    description: "Real-time GAS/NEO price",
    icon: "💹",
    category: "utility",
    entry_url: "/miniapps/builtin/price-ticker/index.html",
    permissions: { datafeed: true },
  },
  {
    app_id: "builtin-secret-vote",
    name: "Secret Vote",
    description: "Vote on governance proposals",
    icon: "🗳️",
    category: "governance",
    entry_url: "/miniapps/builtin/secret-vote/index.html",
    permissions: { governance: true },
  },
];

export default function HomePage() {
  const [wallet, setWallet] = useState<WalletState>({ connected: false, address: "", provider: null });
  const [stats, setStats] = useState<Record<string, MiniAppStats>>({});
  const [notifications, setNotifications] = useState<MiniAppNotification[]>([]);
  const [selectedApp, setSelectedApp] = useState<MiniAppInfo | null>(null);
  const [filter, setFilter] = useState<string>("all");

  useEffect(() => {
    fetchStats();
    fetchNotifications();
  }, []);

  const fetchStats = async () => {
    try {
      const res = await fetch("/api/miniapp-stats");
      const data = await res.json();
      const map: Record<string, MiniAppStats> = {};
      (data.stats || []).forEach((s: MiniAppStats) => {
        map[s.app_id] = s;
      });
      setStats(map);
    } catch (e) {
      console.error("Failed to fetch stats", e);
    }
  };

  const fetchNotifications = async () => {
    try {
      const res = await fetch("/api/miniapp-notifications?limit=10");
      const data = await res.json();
      setNotifications(data.notifications || []);
    } catch (e) {
      console.error("Failed to fetch notifications", e);
    }
  };

  const connectWallet = useCallback(async () => {
    try {
      const g = window as any;
      if (g?.NEOLineN3) {
        const inst = new g.NEOLineN3.Init();
        const acc = await inst.getAccount();
        setWallet({ connected: true, address: acc.address, provider: "neoline" });
      }
    } catch (e) {
      console.error("Wallet connection failed", e);
    }
  }, []);

  const filteredApps = filter === "all" ? MINIAPP_CATALOG : MINIAPP_CATALOG.filter((a) => a.category === filter);

  return (
    <div style={containerStyle}>
      <Header wallet={wallet} onConnect={connectWallet} />
      <main style={mainStyle}>
        <section style={heroSection}>
          <h1 style={heroTitle}>Neo MiniApp Platform</h1>
          <p style={heroDesc}>Discover decentralized apps powered by Neo N3</p>
        </section>

        <div style={contentGrid}>
          <div style={appsSection}>
            <div style={filterRow}>
              {["all", "gaming", "defi", "governance", "utility"].map((cat) => (
                <button key={cat} onClick={() => setFilter(cat)} style={filter === cat ? filterBtnActive : filterBtn}>
                  {cat}
                </button>
              ))}
            </div>
            <div style={appsGrid}>
              {filteredApps.map((app) => (
                <MiniAppCard key={app.app_id} app={app} stats={stats[app.app_id]} onClick={() => setSelectedApp(app)} />
              ))}
            </div>
          </div>

          <aside style={sidebar}>
            <h3 style={sidebarTitle}>📢 Latest News</h3>
            <div style={notificationsList}>
              {notifications.length === 0 ? (
                <p style={{ color: colors.textMuted, fontSize: 14 }}>No notifications yet</p>
              ) : (
                notifications.map((n) => <NotificationCard key={n.id} notification={n} />)
              )}
            </div>
          </aside>
        </div>
      </main>

      {selectedApp && (
        <div style={modalOverlay} onClick={() => setSelectedApp(null)}>
          <div style={modalContent} onClick={(e) => e.stopPropagation()}>
            <div style={{ fontSize: 48 }}>{selectedApp.icon}</div>
            <h2>{selectedApp.name}</h2>
            <p style={{ color: colors.textMuted }}>{selectedApp.description}</p>
            <button style={launchBtn} onClick={() => window.open(selectedApp.entry_url, "_blank")}>
              Launch App
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

const containerStyle: React.CSSProperties = {
  minHeight: "100vh",
  background: colors.bg,
  color: colors.text,
  fontFamily: "system-ui, sans-serif",
};
const mainStyle: React.CSSProperties = { maxWidth: 1200, margin: "0 auto", padding: "0 24px" };
const heroSection: React.CSSProperties = { textAlign: "center", padding: "48px 0 32px" };
const heroTitle: React.CSSProperties = {
  fontSize: 36,
  fontWeight: 700,
  margin: 0,
  background: `linear-gradient(135deg, ${colors.primary}, ${colors.accent})`,
  WebkitBackgroundClip: "text",
  WebkitTextFillColor: "transparent",
};
const heroDesc: React.CSSProperties = { fontSize: 16, color: colors.textMuted, marginTop: 8 };
const contentGrid: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 300px", gap: 32 };
const appsSection: React.CSSProperties = { flex: 1 };
const filterRow: React.CSSProperties = { display: "flex", gap: 8, marginBottom: 20 };
const filterBtn: React.CSSProperties = {
  padding: "8px 16px",
  borderRadius: 8,
  border: `1px solid ${colors.border}`,
  background: "transparent",
  color: colors.textMuted,
  cursor: "pointer",
  textTransform: "capitalize",
};
const filterBtnActive: React.CSSProperties = {
  ...filterBtn,
  background: colors.primary,
  color: "#000",
  borderColor: colors.primary,
};
const appsGrid: React.CSSProperties = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
  gap: 16,
};
const sidebar: React.CSSProperties = { position: "sticky", top: 80, height: "fit-content" };
const sidebarTitle: React.CSSProperties = { fontSize: 16, fontWeight: 600, marginBottom: 16 };
const notificationsList: React.CSSProperties = { display: "flex", flexDirection: "column", gap: 12 };
const modalOverlay: React.CSSProperties = {
  position: "fixed",
  inset: 0,
  background: "rgba(0,0,0,0.8)",
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  zIndex: 1000,
};
const modalContent: React.CSSProperties = {
  background: colors.bgCard,
  borderRadius: 20,
  padding: 32,
  textAlign: "center",
  maxWidth: 400,
};
const launchBtn: React.CSSProperties = {
  marginTop: 20,
  padding: "12px 32px",
  borderRadius: 10,
  border: "none",
  background: colors.primary,
  color: "#000",
  fontWeight: 600,
  cursor: "pointer",
  fontSize: 16,
};
