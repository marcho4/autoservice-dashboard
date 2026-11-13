import { Badge } from "@/components/ui";
import { Bot, BOT_ERRORS } from "@/lib/api";

const tones = { pending: "neutral", running: "ok", error: "danger" } as const;

const labels = { pending: "Запускается…", running: "Работает", error: "Не работает" };

export function BotStatusBadge({ bot }: { bot: Bot }) {
  return (
    <Badge tone={tones[bot.status]}>
      <span className={`size-1.5 rounded-full bg-current ${bot.status === "pending" ? "motion-safe:animate-pulse" : ""}`} />
      {labels[bot.status]}
    </Badge>
  );
}

export function botErrorText(bot: Bot): string {
  return BOT_ERRORS[bot.status_error] ?? "Бот остановился из-за ошибки. Попробуйте подключить токен заново.";
}

// What the owner sends to clients: the bot link with a line that says what it is for.
export function clientInvite(serviceName: string, bot: Bot): string {
  return `Оставить заявку в автосервис «${serviceName}»: ${bot.link}`;
}
