"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";

const links = [{ href: "/archive", label: "Archive" }, { href: "/about", label: "About" }, { href: "/search", label: "Search" }];

export default function SiteHeader() {
  const pathname = usePathname();
  const menu = useRef<HTMLDetailsElement>(null);
  useEffect(() => { if (pathname && menu.current) menu.current.open = false; }, [pathname]);
  const navigation = links.map(({ href, label }) => <Link key={href} href={href} aria-current={pathname === href ? "page" : undefined} onClick={() => { if (menu.current) menu.current.open = false; }}>{label}</Link>);
  return <header className={`site-header${pathname === "/" ? " site-header--home" : ""}`}>
    <Link className="site-brand" href="/" aria-label="iSaber home">iSaber</Link>
    <nav className="site-nav" aria-label="Main">{navigation}</nav>
    <details className="site-mobile-nav" ref={menu} onKeyDown={(event) => {
      if (event.key === "Escape" && menu.current?.open) {
        menu.current.open = false;
        menu.current.querySelector("summary")?.focus();
      }
    }}>
      <summary>Menu</summary><nav aria-label="Mobile">{navigation}</nav>
    </details>
  </header>;
}
