import { CatalogCover } from "@/components/catalog/CatalogCover";
import { validImageURL } from "@/lib/catalogData";

export function RealMangaCover({
  title,
  color,
  coverUrl,
  large = false,
  decorative = false,
}: {
  title: string;
  color: string;
  coverUrl?: string;
  large?: boolean;
  decorative?: boolean;
}) {
  if (validImageURL(coverUrl)) {
    return (
      <img
        className={large ? "real-cover large" : "real-cover"}
        src={coverUrl}
        alt={decorative ? "" : `${title} cover`}
        loading={large ? "eager" : "lazy"}
      />
    );
  }
  return <CatalogCover title={title} color={color} large={large} />;
}
