import type { Metadata } from "next";
import "./globals.css";

const publicOrigin = "https://timothy-unimelb.github.io/Obsidio";

export const metadata: Metadata = {
  metadataBase: new URL(publicOrigin),
  title: "Obsidio — Engineering Under Siege",
  description: "A visual guide to the Obsidio resilience challenge, our Go solution, its performance record, and the testing protocol behind future optimizations.",
  icons: {
    icon: `${publicOrigin}/favicon.svg`,
    shortcut: `${publicOrigin}/favicon.svg`,
  },
  openGraph: {
    title: "Obsidio — Engineering Under Siege",
    description: "See the siege, the bounded-concurrency Go solution, its measured progression, and the protocol used to evaluate future changes.",
    images: [{ url: `${publicOrigin}/og.png`, width: 1536, height: 1024, alt: "Obsidio — Engineering Under Siege" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Obsidio — Engineering Under Siege",
    description: "A visual explanation of the challenge, our Go solution, its performance, and the testing protocol.",
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
