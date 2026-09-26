"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { api, Employee } from "@/lib/api";

const NAV = [
  { href: "/requests", label: "Заявки" },
  { href: "/clients", label: "Клиенты" },
  { href: "/service", label: "Автосервис и бот" },
  { href: "/account", label: "Аккаунт" },
];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [me, setMe] = useState<Employee | null>(null);

  useEffect(() => {
    api.get<Employee>("me").then(setMe).catch(() => undefined);
  }, [pathname]);

  async function logout() {
    await api.logout().catch(() => undefined);
    router.replace("/login");
    router.refresh();
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-zinc-200 bg-white">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
          <Link href="/requests" className="font-semibold">
            🔧 Автосервис
          </Link>
          <nav className="flex flex-1 flex-wrap gap-1">
            {NAV.map((item) => {
              const active = pathname === item.href || pathname.startsWith(item.href + "/");
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`rounded-md px-3 py-1.5 text-sm ${active ? "bg-zinc-900 text-white" : "text-zinc-700 hover:bg-zinc-100"}`}
                >
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <div className="flex items-center gap-3 text-sm">
            {me && <span className="hidden text-zinc-600 sm:inline">{me.full_name}</span>}
            <button onClick={logout} className="rounded-md px-3 py-1.5 text-zinc-700 hover:bg-zinc-100">
              Выйти
            </button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">{children}</main>
    </div>
  );
}
