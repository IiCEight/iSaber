"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

export default function ArticleCode({ children, language }: { children: ReactNode; language: string }) {
  const code = useRef<HTMLDivElement>(null);
  const [ready, setReady] = useState(false);
  const [message, setMessage] = useState("");
  useEffect(() => setReady(true), []);
  return <div className="article-code" ref={code}>
    <div className="article-code-toolbar"><span>{language}</span><button type="button" hidden={!ready} onClick={async () => {
      try {
        await navigator.clipboard.writeText(code.current?.querySelector("code")?.textContent ?? "");
        setMessage("Code copied.");
      } catch { setMessage("Copy is unavailable here. Select the code to copy it manually."); }
    }}>Copy code</button></div>
    {children}
    <p className="article-copy-status" role="status">{message}</p>
  </div>;
}
