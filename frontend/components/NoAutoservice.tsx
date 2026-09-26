import Link from "next/link";
import { Card } from "@/components/ui";

export default function NoAutoservice() {
  return (
    <Card>
      <p className="text-sm text-zinc-700">У вас ещё нет автосервиса. Зарегистрируйте его и подключите Telegram-бота, чтобы получать заявки.</p>
      <Link href="/service" className="mt-4 inline-block rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-700">
        Зарегистрировать автосервис
      </Link>
    </Card>
  );
}
