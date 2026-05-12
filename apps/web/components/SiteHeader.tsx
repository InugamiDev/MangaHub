"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { Bookmark, Crown, List, LogIn, Menu, MessageCircle, Search, Users, X } from "lucide-react";

const navItems = [
  { href: "/#updates", label: "Updates", icon: List },
  { href: "/library", label: "Library", icon: Search },
  { href: "/genres", label: "Genres", icon: List },
  { href: "/popular", label: "Ranking", icon: Crown },
  { href: "/bookmarks", label: "Favorite", icon: Bookmark },
  { href: "/friends", label: "Friends", icon: Users },
  { href: "/chat", label: "Chat", icon: MessageCircle },
  { href: "/login", label: "Login", icon: LogIn },
];

export function SiteHeader() {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    setOpen(false);
  }, [pathname]);

  return (
    <header className="topbar">
      <div className="shell nav">
        <div className="nav-brand-toggle">
          <Link href="/" className="brand" aria-label="MangaHub home" onClick={() => setOpen(false)}>
            <span className="brand-mark">M</span>
            <span>MangaHub</span>
          </Link>
          <button
            type="button"
            className="mobile-menu-button"
            aria-label={open ? "Close navigation menu" : "Open navigation menu"}
            aria-expanded={open}
            aria-controls="main-navigation"
            onClick={() => setOpen((value) => !value)}
          >
            {open ? <X size={22} aria-hidden="true" /> : <Menu size={22} aria-hidden="true" />}
          </button>
        </div>
        <form className="nav-search" action="/search" onSubmit={() => setOpen(false)}>
          <Search size={15} aria-hidden="true" />
          <input name="q" placeholder="Search by title or author" aria-label="Search manga" />
        </form>
        <nav id="main-navigation" className={`nav-links${open ? " is-open" : ""}`} aria-label="Main navigation">
          {navItems.map(({ href, label, icon: Icon }) => (
            <Link href={href} key={href} onClick={() => setOpen(false)}>
              <Icon size={16} aria-hidden="true" />
              {label}
            </Link>
          ))}
        </nav>
      </div>
    </header>
  );
}
