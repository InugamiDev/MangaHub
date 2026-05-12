function renderInline(text: string) {
  const parts = text.split(/(`[^`]+`)/g);
  return parts.map((part, index) => {
    if (part.startsWith("`") && part.endsWith("`")) {
      return <code key={index}>{part.slice(1, -1)}</code>;
    }
    return part;
  });
}

export function MarkdownView({ content }: { content: string }) {
  const lines = content.split(/\r?\n/);
  const nodes: React.ReactNode[] = [];
  let list: string[] = [];
  let code: string[] = [];
  let inCode = false;

  const flushList = () => {
    if (list.length > 0) {
      nodes.push(
        <ul key={`list-${nodes.length}`}>
          {list.map((item) => (
            <li key={item}>{renderInline(item)}</li>
          ))}
        </ul>,
      );
      list = [];
    }
  };

  const flushCode = () => {
    if (code.length > 0) {
      nodes.push(
        <pre key={`code-${nodes.length}`}>
          <code>{code.join("\n")}</code>
        </pre>,
      );
      code = [];
    }
  };

  for (const line of lines) {
    if (line.startsWith("```")) {
      if (inCode) {
        inCode = false;
        flushCode();
      } else {
        flushList();
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      code.push(line);
      continue;
    }
    if (line.startsWith("# ")) {
      flushList();
      nodes.push(<h1 key={`h1-${nodes.length}`}>{line.slice(2)}</h1>);
    } else if (line.startsWith("## ")) {
      flushList();
      nodes.push(<h2 key={`h2-${nodes.length}`}>{line.slice(3)}</h2>);
    } else if (line.startsWith("### ")) {
      flushList();
      nodes.push(<h3 key={`h3-${nodes.length}`}>{line.slice(4)}</h3>);
    } else if (line.startsWith("- ")) {
      list.push(line.slice(2));
    } else if (line.trim() === "") {
      flushList();
    } else {
      flushList();
      nodes.push(<p key={`p-${nodes.length}`}>{renderInline(line)}</p>);
    }
  }
  flushList();
  flushCode();

  return <article className="markdown">{nodes}</article>;
}
