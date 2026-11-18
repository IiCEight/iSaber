"use client";
import { useState } from "react";
import { friendImageURL } from "../lib/friends";

export default function FriendIcon({ name, url, className = "" }: { name: string; url: string; className?: string }) {
  const [failed, setFailed] = useState("");
  const source = friendImageURL(url);
  return <span className={`friend-avatar ${className}`} aria-hidden="true"><span>{Array.from(name.trim()).slice(0, 2).join("").toUpperCase() || "?"}</span>{source && failed !== source && /* biome-ignore lint/performance/noImgElement: User-provided icon URLs must load in the browser, never through a server image proxy. */ <img src={source} alt="" width={96} height={96} loading="lazy" referrerPolicy="no-referrer" onError={() => setFailed(source)} />}</span>;
}
