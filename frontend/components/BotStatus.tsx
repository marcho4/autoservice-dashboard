import { Bot, BOT_ERRORS } from "@/lib/api";

const styles = {
  pending: "bg-amber-100 text-amber-800",
  running: "bg-emerald-100 text-emerald-800",
  error: "bg-red-100 text-red-800",
};

const labels = { pending: "Запускается…", running: "Работает", error: "Не работает" };

export function BotStatusBadge({ bot }: { bot: Bot }) {
  return (
    <span className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ${styles[bot.status]}`}>
      <span className={`h-1.5 w-1.5 rounded-full bg-current ${bot.status === "pending" ? "animate-pulse" : ""}`} />
      {labels[bot.status]}
    </span>
  );
}

export function botErrorText(bot: Bot): string {
  return BOT_ERRORS[bot.status_error] ?? "Бот остановился из-за ошибки. Попробуйте подключить токен заново.";
}
