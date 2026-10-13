import React, { useMemo } from "react";
import { WalletCards } from "lucide-react";

export default function WalletButtons({ walletLinks, appleFallback = "", className = "" }) {
  const links = useMemo(() => normalizedLinks(walletLinks, appleFallback), [walletLinks, appleFallback]);

  if (links.length === 0) return null;

  return (
    <div className={`wallet-actions ${className}`.trim()}>
      {links.map(({ platform, url, label }) => platform === "google" ? (
        <a className="google-wallet-action" href={url} key={platform} target="_blank" rel="noreferrer" aria-label={label}>
          <img src="/add-to-google-wallet.png" alt={label} />
        </a>
      ) : (
        <a className="primary-action apple-wallet-action" href={url} key={platform}>
          <WalletCards size={18} />
          {label}
        </a>
      ))}
    </div>
  );
}

function normalizedLinks(walletLinks, appleFallback) {
  const available = [];
  if (walletLinks?.apple?.url || appleFallback) {
    available.push({
      platform: "apple",
      url: walletLinks?.apple?.url || appleFallback,
      label: walletLinks?.apple?.label || "Add to Apple Wallet",
    });
  }
  if (walletLinks?.google?.url) {
    available.push({
      platform: "google",
      url: walletLinks.google.url,
      label: walletLinks.google.label || "Add to Google Wallet",
    });
  }

  const preferred = devicePlatform();
  return available.sort((left, right) => Number(right.platform === preferred) - Number(left.platform === preferred));
}

function devicePlatform() {
  if (typeof navigator === "undefined") return "apple";
  const userAgent = navigator.userAgent || "";
  if (/android/i.test(userAgent)) return "google";
  if (/iPhone|iPad|iPod/i.test(userAgent)) return "apple";
  return "apple";
}
