import type { CSSProperties } from "react";

export function CatalogCover({ title, color, large = false }: { title: string; color: string; large?: boolean }) {
  return (
    <div className={large ? "catalog-cover large" : "catalog-cover"} style={{ "--hue": color } as CSSProperties}>
      <span>{title.slice(0, 2).toUpperCase()}</span>
    </div>
  );
}
