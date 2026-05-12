export function PageTitle({ eyebrow, title, copy }: { eyebrow?: string; title: string; copy?: string }) {
  return (
    <header className="page-title">
      {eyebrow ? <p className="kicker">{eyebrow}</p> : null}
      <h1>{title}</h1>
      {copy ? <p>{copy}</p> : null}
    </header>
  );
}
