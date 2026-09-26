import React, { useEffect, useMemo } from "react";
import {
  Building2,
  Coffee,
  Gift,
  Heart,
  ImageUp,
  ShoppingBag,
  Star,
} from "lucide-react";

const stampIcons = [
  { key: "logo", label: "Merchant logo", icon: Building2 },
  { key: "custom", label: "Custom icon", icon: ImageUp },
];

const legacyIcons = { coffee: Coffee, star: Star, gift: Gift, heart: Heart, shopping_bag: ShoppingBag };

export default function StampArtworkPicker({ value, onChange, logoFile, customFile, sourceURL, onCustomFile, goal = 10, progress = 6, disabled = false }) {
  const legacySelection = value === "none" || Boolean(legacyIcons[value]);
  return (
    <div className="stamp-artwork-picker">
      {legacySelection ? <p className="stamp-legacy-note">This pass currently uses a legacy stamp setting. Choose the merchant logo or a custom icon to enable the physical stamp card.</p> : null}
      <div className="stamp-picker-options" role="radiogroup" aria-label="Stamp artwork">
        {stampIcons.map((option) => {
          const Icon = option.icon;
          return (
            <button
              key={option.key}
              type="button"
              className={value === option.key ? "selected" : ""}
              onClick={() => onChange(option.key)}
              disabled={disabled}
              role="radio"
              aria-checked={value === option.key}
              title={option.label}
            >
              <Icon size={19} />
              <span>{option.label}</span>
            </button>
          );
        })}
      </div>

      {value === "custom" ? (
        <label className="stamp-custom-upload">
          <span>Custom transparent PNG</span>
          <input type="file" accept="image/png" onChange={(event) => onCustomFile(event.target.files?.[0] ?? null)} disabled={disabled} />
          <small>Square, 64-512 px, up to 1 MB.</small>
        </label>
      ) : null}

      <StampRowPreview iconKey={value} sourceFile={value === "custom" ? customFile : logoFile} sourceURL={sourceURL} goal={goal} progress={progress} />
      <p className="stamp-picker-note">Use artwork with strong contrast against white. Visual stamps supplement the numeric counter and may not appear on Apple Watch or every Wallet layout.</p>
    </div>
  );
}

export function StampRowPreview({ iconKey, sourceFile, sourceURL = "", goal = 10, progress = 6 }) {
  const selected = stampIcons.find((option) => option.key === iconKey);
  const PreviewIcon = selected?.icon ?? legacyIcons[iconKey] ?? Building2;
  const localPreviewURL = useMemo(() => sourceFile ? URL.createObjectURL(sourceFile) : "", [sourceFile]);

  useEffect(() => () => {
    if (localPreviewURL) URL.revokeObjectURL(localPreviewURL);
  }, [localPreviewURL]);

  const visualGoal = Number(goal);
  const showPreview = iconKey !== "none" && visualGoal > 0 && visualGoal <= 10;
  const filledCount = Math.min(Math.max(Number(progress) || 0, 0), visualGoal || 0);
  const artworkURL = localPreviewURL || sourceURL;

  return (
    <div className={`stamp-row-preview ${showPreview ? "" : "numeric-only"}`} aria-label="Stamp artwork preview">
      {showPreview ? Array.from({ length: visualGoal }, (_, index) => (
        <span key={index} className={index < filledCount ? "filled" : "empty"}>
          {index < filledCount ? (artworkURL ? <img src={artworkURL} alt="" /> : <PreviewIcon size={20} />) : null}
        </span>
      )) : <p>{visualGoal > 10 ? "Goals above 10 use the numeric stamp counter." : "The pass will use its numeric stamp counter only."}</p>}
    </div>
  );
}
