import Link from "next/link";
import { buttonClass, Empty } from "@/components/ui";

export default function NoAutoservice() {
  return (
    <Empty
      title="У вас ещё нет автосервиса"
      actions={
        <Link href="/onboarding" className={buttonClass()}>
          Зарегистрировать автосервис
        </Link>
      }
    >
      Зарегистрируйте его и подключите Telegram-бота, чтобы получать заявки.
    </Empty>
  );
}
