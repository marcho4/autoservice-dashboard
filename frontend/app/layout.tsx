import type { Metadata, Viewport } from "next";
import { Golos_Text } from "next/font/google";
import "./globals.css";

const golos = Golos_Text({
  variable: "--font-golos",
  subsets: ["latin", "cyrillic"],
});

export const metadata: Metadata = {
  title: "Автосервис — заявки",
  description: "Дашборд заявок автосервиса из Telegram",
};

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
