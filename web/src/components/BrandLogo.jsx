import React from "react";

const LOGOS = {
  primary: "/brand/just-loyalty/just-loyalty-primary.svg",
  dark: "/brand/just-loyalty/just-loyalty-primary-dark.svg",
  icon: "/brand/just-loyalty/just-loyalty-icon.svg",
  mono: "/brand/just-loyalty/just-loyalty-icon-mono.svg",
};

export default function BrandLogo({ variant = "primary", className = "", alt = "Just Loyalty" }) {
  return (
    <img
      className={`brand-logo ${className}`.trim()}
      src={LOGOS[variant] ?? LOGOS.primary}
      alt={alt}
    />
  );
}
