import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Relay",
  description: "Real-time messaging — ordering, reconnect, presence demo",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
