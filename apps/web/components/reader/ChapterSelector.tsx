"use client";

import { useRouter } from "next/navigation";

export function ChapterSelector({
  mangaSlug,
  chapters,
  currentChapterId,
}: {
  mangaSlug: string;
  chapters: { id: string; number: number }[];
  currentChapterId: string;
}) {
  const router = useRouter();

  return (
    <select
      defaultValue={currentChapterId}
      aria-label="Chapter selector"
      onChange={(event) => router.push(`/manga/${mangaSlug}/chapter/${event.target.value}`)}
    >
      {chapters.map((item) => (
        <option value={item.id} key={item.id}>Chapter {item.number}</option>
      ))}
    </select>
  );
}
