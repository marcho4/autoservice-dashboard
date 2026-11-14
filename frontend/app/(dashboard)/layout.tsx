"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { ReactNode, useEffect, useState } from "react";
import { botErrorText } from "@/components/BotStatus";
import { Button, buttonClass, Logo } from "@/components/ui";
import { api, Autoservice, Employee } from "@/lib/api";
import { useBot } from "@/lib/useBot";

const NAV = [
  { href: "/requests", label: "Заявки" },
  { href: "/clients", label: "Клиенты" },
  { href: "/services", label: "Услуги" },
  { href: "/service", label: "Автосервис и бот", short: "Настройки" },
  { href: "/account", label: "Аккаунт" },
];

function Banner({ tone, children, action }: { tone: "accent" | "danger"; children: ReactNode; action: { href: string; label: string } }) {
  return (
    <div role={tone === "danger" ? "alert" : "status"} className={`border-b ${tone === "danger" ? "border-danger/30 bg-danger-soft text-danger" : "border-line-strong bg-accent-soft"}`}>
      <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2 font-medium">
        <span className="min-w-0 flex-1 basis-64">{children}</span>
        <Link href={action.href} className={buttonClass({ variant: "secondary", size: "sm" })}>
          {action.label}
        </Link>
      </div>
    </div>
  );
}

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<Employee | null>(null);
  const [serviceName, setServiceName] = useState("");
  const { bot } = useBot(20000);

  useEffect(() => {
    api.get<Employee>("me").then(setMe).catch(() => undefined);
    api
      .get<Autoservice>("autoservice")
      .then((s) => setServiceName(s.name))
      .catch(() => setServiceName(""));
  }, [pathname]);

  async function logout() {
    await api.logout().catch(() => undefined);
    router.replace("/login");
    router.refresh();
  }

  const showBotBanner = pathname !== "/service";

  const links = (variant: "top" | "bottom") =>
    NAV.map((item) => {
      const active = pathname === item.href || pathname.startsWith(item.href + "/");
      return (
        <Link
          key={item.href}
          href={item.href}
          aria-current={active ? "page" : undefined}
          className={
            variant === "top"
              ? `flex h-full items-center border-b-2 px-3 whitespace-nowrap ${active ? "border-ink font-semibold" : "border-transparent text-ink-2 hover:text-ink"}`
              : `flex min-w-0 flex-1 items-center justify-center border-t-2 text-tab active:bg-sunken ${active ? "border-ink font-semibold" : "border-transparent text-ink-2"}`
          }
        >
          {variant === "bottom" ? (item.short ?? item.label) : item.label}
        </Link>
      );
    });

  return (
    <div className="min-h-dvh">
      <header className="border-b border-line bg-surface">
        <div className="mx-auto flex h-14 max-w-6xl items-center gap-4 px-4">
          <Link href="/requests" className="flex h-full min-w-0 items-center gap-2.5 font-semibold">
            <Logo />
            <span className="truncate">{serviceName || "Автосервис"}</span>
          </Link>
          <nav aria-label="Разделы" className="ml-2 hidden h-full flex-1 sm:flex">
            {links("top")}
          </nav>
          <div className="ml-auto flex shrink-0 items-center gap-1">
            {me && <span className="hidden text-meta text-ink-2 lg:inline">{me.full_name}</span>}
            <Button variant="ghost" size="sm" onClick={logout}>
              Выйти
            </Button>
          </div>
        </div>
      </header>

      {showBotBanner && bot === null && (
        <Banner tone="accent" action={{ href: "/onboarding", label: "Подключить бота" }}>
          Бот ещё не подключён — клиенты пока не могут оставлять заявки.
        </Banner>
      )}
      {showBotBanner && bot?.status === "error" && (
        <Banner tone="danger" action={{ href: "/service", label: "Исправить" }}>
          Бот @{bot.username} не принимает заявки. {botErrorText(bot)}
        </Banner>
      )}

      <main className="mx-auto max-w-6xl px-4 pt-6 pb-[calc(var(--spacing-tabbar)+env(safe-area-inset-bottom)+2rem)] sm:pt-8 sm:pb-12">{children}</main>

      <nav aria-label="Разделы" className="fixed inset-x-0 bottom-0 z-20 border-t border-line bg-surface pb-[env(safe-area-inset-bottom)] sm:hidden">
        <div className="flex h-tabbar">{links("bottom")}</div>
      </nav>
    </div>
  );
}
