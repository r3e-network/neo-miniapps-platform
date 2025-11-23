import { useState } from "react";

type Wallet = { address: string; label?: string; signature?: string };

type Props = {
  wallet: Wallet;
  onConnect: (wallet: Wallet) => void;
  onDisconnect: () => void;
};

export function WalletGate({ wallet, onConnect, onDisconnect }: Props) {
  const [address, setAddress] = useState(wallet.address ?? "");
  const [label, setLabel] = useState(wallet.label ?? "");
  const [signature, setSignature] = useState(wallet.signature ?? "");

  const connected = Boolean(wallet.address);

  return (
    <section className="card inner">
      <div className="row">
        <h3>Neo wallet login</h3>
        {connected && (
          <span className="tag subdued">
            Connected {wallet.label ? `(${wallet.label})` : ""} {wallet.address}
          </span>
        )}
      </div>
      <p className="muted">
        Dashboard accepts token auth and optional Neo wallet connect. Paste your N3 address (and an optional signed note) to mark this session as
        yours. This is a client-side convenience; keep API tokens private.
      </p>
      {!connected && (
        <div className="form-grid">
          <input value={address} onChange={(e) => setAddress(e.target.value)} placeholder="Neo N3 address" />
          <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder="Label (optional)" />
          <input value={signature} onChange={(e) => setSignature(e.target.value)} placeholder="Signed note (optional)" />
          <button
            type="button"
            onClick={() => {
              onConnect({ address: address.trim(), label: label.trim(), signature: signature.trim() });
            }}
            disabled={!address.trim()}
          >
            Connect wallet
          </button>
        </div>
      )}
      {connected && (
        <button type="button" className="ghost" onClick={() => onDisconnect()}>
          Disconnect
        </button>
      )}
    </section>
  );
}
