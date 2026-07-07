import type { Metadata } from "next";
import { Syne, Hanken_Grotesk, Space_Mono } from "next/font/google";
import "./globals.css";

const syne = Syne({
  variable: "--font-display",
  subsets: ["latin"],
  weight: ["600", "700", "800"],
});

const hanken = Hanken_Grotesk({
  variable: "--font-sans",
  subsets: ["latin"],
});

const spaceMono = Space_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
  weight: ["400", "700"],
});

export const metadata: Metadata = {
  title: "Relay — real-time messaging",
  description: "Real-time messaging with gap-free ordering, live presence, and instant reconnect.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${syne.variable} ${hanken.variable} ${spaceMono.variable}`}>
      <body>{children}</body>
    </html>
  );
}
