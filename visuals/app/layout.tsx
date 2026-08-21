import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

export const metadata: Metadata = {
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_SITE_URL ??
      "https://obsidio-engineering-under-siege.timothymathews.chatgpt.site",
  ),
  title: "Obsidio — Engineering Under Siege",
  description: "A visual guide to the Obsidio resilience challenge and our high-throughput Go solution.",
  icons: {
    icon: "/favicon.svg",
    shortcut: "/favicon.svg",
  },
  openGraph: {
    title: "Obsidio — Engineering Under Siege",
    description: "See the siege, the bottleneck, and the bounded-concurrency Go solution at a glance.",
    images: [{ url: "/og.png", width: 1536, height: 1024, alt: "Obsidio — Engineering Under Siege" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Obsidio — Engineering Under Siege",
    description: "A visual explanation of the challenge and our high-throughput Go solution.",
    images: ["/og.png"],
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body className={`${geistSans.variable} ${geistMono.variable}`}>{children}</body>
    </html>
  );
}
