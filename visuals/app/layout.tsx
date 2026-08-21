import type { Metadata } from "next";
import "./globals.css";

const publicOrigin = "https://timothy-unimelb.github.io/Obsidio";

export const metadata: Metadata = {
  metadataBase: new URL(publicOrigin),
  title: "Obsidio — Engineering Under Siege",
  description: "A visual guide to the Obsidio resilience challenge, our high-throughput Go solution, and the next optimization roadmap.",
  icons: {
    icon: `${publicOrigin}/favicon.svg`,
    shortcut: `${publicOrigin}/favicon.svg`,
  },
  openGraph: {
    title: "Obsidio — Engineering Under Siege",
    description: "See the siege, the bounded-concurrency Go solution, and the evidence-led optimization roadmap at a glance.",
    images: [{ url: `${publicOrigin}/og.png`, width: 1536, height: 1024, alt: "Obsidio — Engineering Under Siege" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Obsidio — Engineering Under Siege",
    description: "A visual explanation of the challenge, our high-throughput Go solution, and the next optimization roadmap.",
    images: [`${publicOrigin}/og.png`],
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
