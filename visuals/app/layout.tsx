import type { Metadata } from "next";
import "./globals.css";

const publicOrigin = "https://timothy-unimelb.github.io/Obsidio";

export const metadata: Metadata = {
  metadataBase: new URL(publicOrigin),
  title: "Obsidio — Engineering Under Siege",
  description: "A visual guide to the Obsidio resilience challenge and our high-throughput Go solution.",
  icons: {
    icon: `${publicOrigin}/favicon.svg`,
    shortcut: `${publicOrigin}/favicon.svg`,
  },
  openGraph: {
    title: "Obsidio — Engineering Under Siege",
    description: "See the siege, the bottleneck, and the bounded-concurrency Go solution at a glance.",
    images: [{ url: `${publicOrigin}/og.png`, width: 1536, height: 1024, alt: "Obsidio — Engineering Under Siege" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Obsidio — Engineering Under Siege",
    description: "A visual explanation of the challenge and our high-throughput Go solution.",
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
