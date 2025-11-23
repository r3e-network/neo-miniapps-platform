import "./notifications.css";

export type Notification = { id: string; type: "success" | "error"; message: string };

type Props = { items: Notification[]; onDismiss: (id: string) => void };

export function Notifications({ items, onDismiss }: Props) {
  if (!items.length) return null;
  return (
    <div className="notifications">
      {items.map((n) => (
        <div key={n.id} className={`notification ${n.type}`}>
          <span>{n.message}</span>
          <button aria-label="dismiss" onClick={() => onDismiss(n.id)}>
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
