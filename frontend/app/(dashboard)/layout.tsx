"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { botErrorText } from "@/components/BotStatus";
import { api, Employee } from "@/lib/api";
import { useBot } from "@/lib/useBot";

const NAV = [
  { href: "/requests", label: "Заявки" },
  { href: "/clients", label: "Клиенты" },
  { href: "/services", label: "Услуги" },
  { href: "/service", label: "Автосервис и бот" },
  { href: "/account", label: "Аккаунт" },
];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<Employee | null>(null);
  const { bot } = useBot(20000);

  useEffect(() => {
    api.get<Employee>("me").then(setMe).catch(() => undefined);
  }, [pathname]);

  async function logout() {
    await api.logout().catch(() => undefined);
    router.replace("/login");
    router.refresh();
  }

  // The settings page shows the bot state itself.
  const showBotBanner = pathname !== "/service";

  return (
    <div className="min-h-screen">
      <header className="border-b border-zinc-200 bg-white">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
          <Link href="/requests" className="font-semibold">
            🔧 Автосервис
          </Link>
          {/* On a phone the menu is its own scrollable row under the brand, so it never wraps into a column. */}
          <nav className="order-last flex w-full gap-1 overflow-x-auto sm:order-none sm:w-auto sm:flex-1 sm:flex-wrap">
            {NAV.map((item) => {
              const active = pathname === item.href || pathname.startsWith(item.href + "/");
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`whitespace-nowrap rounded-md px-3 py-1.5 text-sm ${active ? "bg-zinc-900 text-white" : "text-zinc-700 hover:bg-zinc-100"}`}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <div className="ml-auto flex items-center gap-3 text-sm">
            {me && <span className="hidden text-zinc-600 sm:inline">{me.full_name}</span>}
            <button onClick={logout} className="rounded-md px-3 py-1.5 text-zinc-700 hover:bg-zinc-100">
              Выйти
            </button>
          </div>
        </div>
      </header>

      {showBotBanner && bot === null && (
        <div className="border-b border-amber-200 bg-amber-50">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-2.5 text-sm text-amber-900">
            <span>Бот ещё не подключён — клиенты пока не могут оставлять заявки.</span>
            <Link href="/onboarding" className="rounded-md bg-amber-900 px-3 py-1 font-medium text-white hover:bg-amber-800">
              Подключить бота
            </Link>
          </div>
        </div>
      )}
      {showBotBanner && bot?.status === "error" && (
        <div className="border-b border-red-200 bg-red-50">
          <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-4 py-2.5 text-sm text-red-900">
            <span>Бот @{bot.username} не принимает заявки. {botErrorText(bot)}</span>
            <Link href="/service" className="rounded-md bg-red-800 px-3 py-1 font-medium text-white hover:bg-red-700">
              Исправить
            </Link>
          </div>
        </div>
      )}

      <main className="mx-auto max-w-6xl px-4 py-8">{children}</main>
    </div>
  );
}
