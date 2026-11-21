"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";
import { ThemeToggle } from "./theme-provider";

const links = [{ href: "/archive", label: "Archive" }, { href: "/friends", label: "Friends" }, { href: "/about", label: "About" }, { href: "/archive#archive-search-panel", label: "Search" }];

export default function SiteHeader() {
  const pathname = usePathname();
  const menu = useRef<HTMLDetailsElement>(null);
  useEffect(() => { if (pathname && menu.current) menu.current.open = false; }, [pathname]);
  if (pathname === "/admin" || pathname.startsWith("/admin/")) return null;
  const navigation = links.map(({ href, label }) => <Link key={href} href={label === "Search" && pathname === "/archive" ? "#archive-search-panel" : href} aria-current={pathname === href ? "page" : undefined} onClick={event => {
    if (menu.current) menu.current.open = false;
    if (label === "Search" && pathname === "/archive" && event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey) {
      // Open the existing Archive controls without dropping applied conditions.
      event.preventDefault();
      window.dispatchEvent(new Event("isaber:open-archive-search"));
    }
  }}>{label}</Link>);
  return <header className={`site-header${pathname === "/" ? " site-header--home" : pathname === "/about" ? " site-header--about" : ""}`}>
    <Link className="site-brand" href="/" aria-label="iSaber home">iSaber</Link>
    <div className="site-header-actions">
      <nav className="site-nav" aria-label="Main">{navigation}</nav>
      <a className="site-github" href="https://github.com/iiiiSaber/iSaber" target="_blank" rel="noopener noreferrer" title="View source on GitHub">
        <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor" aria-hidden="true"><path d="M12 .75a11.25 11.25 0 0 0-3.558 21.923c.563.104.77-.244.77-.542 0-.267-.01-1.155-.015-2.095-3.13.68-3.79-1.327-3.79-1.327-.512-1.3-1.25-1.646-1.25-1.646-1.022-.699.077-.685.077-.685 1.13.08 1.725 1.16 1.725 1.16 1.005 1.723 2.637 1.225 3.28.937.102-.728.394-1.225.716-1.506-2.499-.285-5.126-1.25-5.126-5.563 0-1.229.44-2.232 1.16-3.02-.117-.285-.503-1.429.11-2.978 0 0 .945-.302 3.095 1.153A10.79 10.79 0 0 1 12 6.18c.956.004 1.92.13 2.82.38 2.148-1.456 3.092-1.153 3.092-1.153.614 1.55.228 2.693.112 2.977.722.789 1.158 1.792 1.158 3.021 0 4.324-2.632 5.275-5.139 5.554.404.35.765 1.04.765 2.096 0 1.513-.014 2.733-.014 3.104 0 .3.203.65.774.54A11.25 11.25 0 0 0 12 .75Z" /></svg>
        <span className="site-github-label">iSaber on GitHub (opens in a new tab)</span>
      </a>
      <ThemeToggle />
      <details className="site-mobile-nav" ref={menu} onKeyDown={(event) => {
        if (event.key === "Escape" && menu.current?.open) {
          menu.current.open = false;
          menu.current.querySelector("summary")?.focus();
        }
      }}>
        <summary>Menu</summary><nav aria-label="Mobile">{navigation}</nav>
      </details>
    </div>
  </header>;
}
