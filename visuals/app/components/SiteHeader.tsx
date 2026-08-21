import Link from "next/link";

export function SiteHeader({ active }: { active: "challenge" | "solution" | "optimizations" | "performance" | "testing" }) {
  return (
    <header className="topbar">
      <Link className="brand" href="/" aria-label="Obsidio challenge home">
        <span className="brandMark">O</span>
        <span>OBSIDIO</span>
      </Link>
      <nav aria-label="Primary navigation">
        <Link className={`navLink ${active === "challenge" ? "active" : ""}`} href="/">
          The challenge
        </Link>
        <Link className={`navLink ${active === "solution" ? "active" : ""}`} href="/solution">
          Our solution
        </Link>
        <Link className={`navLink ${active === "optimizations" ? "active" : ""}`} href="/optimizations">
          Next gains
        </Link>
        <Link className={`navLink ${active === "performance" ? "active" : ""}`} href="/performance">
          Performance
        </Link>
        <Link className={`navLink ${active === "testing" ? "active" : ""}`} href="/testing">
          Testing <span aria-hidden="true">↗</span>
        </Link>
      </nav>
    </header>
  );
}
