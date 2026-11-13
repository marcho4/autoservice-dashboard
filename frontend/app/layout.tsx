import type { Metadata, Viewport } from "next";
import { Golos_Text } from "next/font/google";
import "./globals.css";

// Golos Text is drawn for Cyrillic interfaces: open, sturdy letters that stay readable on a phone held at arm's length.
const golos = Golos_Text({
  variable: "--font-golos",
  subsets: ["latin", "cyrillic"],
});

export const metadata: Metadata = {
  title: "Автосервис — заявки",
  description: "Дашборд заявок автосервиса из Telegram",
};

// viewportFit lets the bottom navigation account for the iPhone home indicator through env(safe-area-inset-*).
export const viewport: Viewport = {
  viewportFit: "cover",
  themeColor: "#f3f2ee",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ru" className={`${golos.variable} h-full antialiased`}>
      <body className="min-h-full">{children}</body>
    </html>
  );
}
