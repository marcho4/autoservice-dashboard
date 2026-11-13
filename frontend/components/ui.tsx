"use client";

import Link from "next/link";
import {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  ReactNode,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
  useEffect,
  useId,
  useState,
} from "react";
import { formatDate, formatPhone, formatWhen, RequestStatus, STATUS_LABELS, telHref } from "@/lib/api";

const ICONS = {
  phone:
    "M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z",
  up: "M12 19V5M5 12l7-7 7 7",
  down: "M12 5v14M19 12l-7 7-7-7",
  back: "M15 18l-6-6 6-6",
  close: "M18 6 6 18M6 6l12 12",
  check: "M20 6 9 17l-5-5",
};

// Icons are decoration next to a text label or inside a button that carries its own aria-label.
export function Icon({ name, className = "size-4" }: { name: keyof typeof ICONS; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={`shrink-0 ${className}`}
    >
      <path d={ICONS[name]} />
    </svg>
  );
}

// The product mark: a hex nut. Replaces the wrench emoji.
export function Logo({ className = "size-7" }: { className?: string }) {
  return (
    <svg viewBox="0 0 28 28" aria-hidden="true" className={`shrink-0 ${className}`}>
      <rect width="28" height="28" rx="7" className="fill-ink" />
      <path
        fillRule="evenodd"
        d="M14 5.5l7.36 4.25v8.5L14 22.5l-7.36-4.25v-8.5L14 5.5zm0 5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7z"
        className="fill-accent"
      />
    </svg>
  );
}

const buttonVariants = {
  primary: "bg-accent text-ink hover:bg-accent-hover active:bg-accent-hover",
  secondary: "border border-line-strong bg-surface text-ink hover:bg-paper active:bg-sunken",
  ghost: "text-ink-2 hover:bg-sunken hover:text-ink active:bg-sunken",
  danger: "border border-danger/40 bg-surface text-danger hover:bg-danger-soft active:bg-danger-soft",
  dangerSolid: "bg-danger text-surface hover:bg-danger-hover active:bg-danger-hover",
};

const buttonSizes = {
  md: "min-h-control px-4 text-body",
  sm: "min-h-control-sm px-3 text-meta",
  // A square icon button: pass an aria-label.
  icon: "size-control-sm",
};

type ButtonLook = { variant?: keyof typeof buttonVariants; size?: keyof typeof buttonSizes };

// Classes of a button, for links that have to look like one. The press answers at once: the scale has no delay.
export function buttonClass({ variant = "primary", size = "md" }: ButtonLook = {}, className = "") {
  return `inline-flex shrink-0 items-center justify-center gap-2 rounded-control py-1.5 text-center leading-tight font-semibold transition-[background-color,color,scale] duration-150 ease-out select-none motion-safe:active:scale-[0.97] disabled:scale-100 disabled:cursor-not-allowed disabled:border-transparent disabled:bg-sunken disabled:text-ink-3 ${buttonVariants[variant]} ${buttonSizes[size]} ${className}`;
}

export function Button({ variant, size, className, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & ButtonLook) {
  return <button className={buttonClass({ variant, size }, className)} {...props} />;
}

// A link inside a sentence.
export const textLinkClass = "font-medium text-ink underline decoration-line-input underline-offset-4 hover:decoration-ink";

// A standalone text link: tall enough for a finger.
export const linkClass = `inline-flex min-h-control-sm items-center ${textLinkClass}`;

export function IconButton({ label, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { label: string }) {
  return <Button type="button" variant="secondary" size="icon" title={label} aria-label={label} {...props} />;
}

// A group of buttons: full-width and stacked under a thumb, in a row where there is room.
export function Actions({ children }: { children: ReactNode }) {
  return <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">{children}</div>;
}

// The main action of the product: one tap dials the client.
export function CallButton({ phone, name, large, className }: { phone: string; name: string; large?: boolean; className?: string }) {
  const number = <span className="font-semibold tabular-nums">{formatPhone(phone)}</span>;
  return (
    <a
      href={telHref(phone)}
      aria-label={`Позвонить: ${name}, ${formatPhone(phone)}`}
      className={buttonClass({ variant: large ? "primary" : "secondary" }, className)}
    >
      <Icon name="phone" className={large ? "size-5" : "size-4"} />
      {large ? (
        <span className="flex flex-col items-start py-1.5">
          <span className="text-meta font-medium">Позвонить</span>
          <span className="text-heading">{number}</span>
        </span>
      ) : (
        number
      )}
    </a>
  );
}

// Copies a text and says so on the button itself.
export function CopyButton({ text, children }: { text: string; children: ReactNode }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const id = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(id);
  }, [copied]);

  async function copy() {
    await navigator.clipboard.writeText(text).catch(() => undefined);
    setCopied(true);
  }

  return (
    <Button type="button" variant="secondary" onClick={copy}>
      {copied && <Icon name="check" />}
      <span aria-live="polite">{copied ? "Скопировано" : children}</span>
    </Button>
  );
}

const inputClass =
  "block min-h-control w-full rounded-control border border-line-input bg-surface px-3 py-2 text-body text-ink focus-visible:border-ink focus-visible:outline-offset-0 aria-invalid:border-danger";

const labelClass = "mb-1.5 block text-meta font-medium text-ink-2";

function Hint({ id, invalid, children }: { id: string; invalid?: boolean; children: ReactNode }) {
  return (
    <span id={id} className={`mt-1.5 block text-meta ${invalid ? "font-medium text-danger" : "text-ink-3"}`}>
      {children}
    </span>
  );
}

type FieldExtras = { label: string; hint?: ReactNode };

export function Field({ label, hint, className = "", ...props }: InputHTMLAttributes<HTMLInputElement> & FieldExtras) {
  const hintId = useId();
  return (
    <label className="block">
      <span className={labelClass}>{label}</span>
      <input className={`${inputClass} ${className}`} aria-describedby={hint ? hintId : undefined} {...props} />
      {hint && <Hint id={hintId}>{hint}</Hint>}
    </label>
  );
}

// invalid marks the field and turns the hint into the reason, before anything is submitted.
export function TextArea({ label, hint, invalid, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement> & FieldExtras & { invalid?: boolean }) {
  const hintId = useId();
  return (
    <label className="block">
      <span className={labelClass}>{label}</span>
      <textarea className={inputClass} rows={3} aria-invalid={invalid || undefined} aria-describedby={hint ? hintId : undefined} {...props} />
      {hint && (
        <Hint id={hintId} invalid={invalid}>
          {hint}
        </Hint>
      )}
    </label>
  );
}

export function Select({ label, ...props }: SelectHTMLAttributes<HTMLSelectElement> & { label: string }) {
  return (
    <label className="block">
      <span className={labelClass}>{label}</span>
      <select className={inputClass} {...props} />
    </label>
  );
}

const panelClass = "rounded-panel border bg-surface";

// A list of rows on one surface, split by hairlines instead of a card per row.
export const listClass = `divide-y divide-line overflow-hidden border-line ${panelClass}`;

// A link stretched over its row (the row is `relative`): the whole card opens, and darkens under the finger.
// Buttons inside the row go above it with `relative z-10`.
export const rowLinkClass = "block text-heading after:absolute after:inset-0 active:after:bg-ink/5";

// Desktop table; a phone gets cards instead (see the pages).
export function Table({ children }: { children: ReactNode }) {
  return (
    <div className={`hidden overflow-x-auto border-line sm:block ${panelClass}`}>
      <table className="w-full text-left [&_tbody_tr]:border-t [&_tbody_tr]:border-line [&_tbody_tr]:hover:bg-paper [&_td]:px-3 [&_td]:py-3 [&_td]:align-top [&_th]:px-3 [&_th]:py-2.5 [&_th]:text-meta [&_th]:font-medium [&_th]:text-ink-3 [&_tr>*:first-child]:pl-5 [&_tr>*:last-child]:pr-5">
        {children}
      </table>
    </div>
  );
}

const cardOutlines = {
  line: "border-line",
  // The card being edited.
  ink: "border-ink",
  // Something switched off: told apart by shape, not by transparency.
  dashed: "border-dashed border-line-strong",
};

export function Card({
  title,
  children,
  actions,
  outline = "line",
  className = "",
}: {
  title?: string;
  children: ReactNode;
  actions?: ReactNode;
  outline?: keyof typeof cardOutlines;
  className?: string;
}) {
  return (
    <section className={`p-4 sm:p-5 ${panelClass} ${cardOutlines[outline]} ${className}`}>
      {(title || actions) && (
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          {title && <h2 className="text-heading">{title}</h2>}
          {actions}
        </div>
      )}
      {children}
    </section>
  );
}

const alertStyles = {
  error: "border-danger/30 bg-danger-soft text-danger",
  success: "border-ok/30 bg-ok-soft text-ok",
};

// Errors interrupt a screen reader, confirmations wait for a pause.
export function Alert({ kind = "error", children }: { kind?: keyof typeof alertStyles; children: ReactNode }) {
  return (
    <div role={kind === "error" ? "alert" : "status"} className={`flex gap-2 rounded-control border px-3 py-2.5 font-medium ${alertStyles[kind]}`}>
      <Icon name={kind === "error" ? "close" : "check"} className="mt-1 size-4" />
      <span className="min-w-0">{children}</span>
    </div>
  );
}

const badgeTones = {
  accent: "bg-accent text-ink",
  outline: "bg-surface text-ink ring-1 ring-line-strong ring-inset",
  neutral: "bg-sunken text-ink-2",
  ok: "bg-ok-soft text-ok",
  danger: "bg-danger-soft text-danger",
};

export function Badge({ tone, children }: { tone: keyof typeof badgeTones; children: ReactNode }) {
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-tag px-2 py-0.5 text-meta font-semibold whitespace-nowrap ${badgeTones[tone]}`}>
      {children}
    </span>
  );
}

// Only a new request wears the accent: it is the one waiting for the master.
const statusTones: Record<RequestStatus, keyof typeof badgeTones> = {
  new: "accent",
  in_progress: "outline",
  closed: "ok",
  rejected: "neutral",
  cancelled: "neutral",
};

export function StatusBadge({ status }: { status: RequestStatus }) {
  return (
    <Badge tone={statusTones[status]}>
      {status === "closed" && <Icon name="check" className="size-3.5" />}
      {STATUS_LABELS[status]}
    </Badge>
  );
}

// "сегодня, 21:48" for the eye, the full date in the tooltip.
export function When({ iso, className }: { iso: string; className?: string }) {
  return (
    <time dateTime={iso} title={formatDate(iso)} className={`whitespace-nowrap ${className ?? ""}`}>
      {formatWhen(iso)}
    </time>
  );
}

export function PageTitle({
  children,
  actions,
  back,
  lead,
}: {
  children: ReactNode;
  actions?: ReactNode;
  back?: { href: string; label: string };
  lead?: ReactNode;
}) {
  return (
    <header className="mb-5">
      {back && (
        <Link href={back.href} className="-ml-1 mb-1 inline-flex min-h-control-sm items-center gap-1 rounded-control pr-2 text-meta font-medium text-ink-2 hover:text-ink">
          <Icon name="back" />
          {back.label}
        </Link>
      )}
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
        <h1 className="min-w-0 text-title text-balance">{children}</h1>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
      {lead && <div className="mt-2 max-w-[65ch] text-ink-2">{lead}</div>}
    </header>
  );
}

// Label over value: request answers, contacts, bot details.
export function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-meta text-ink-3">{label}</dt>
      <dd className="mt-0.5 font-medium wrap-break-word whitespace-pre-wrap">{children}</dd>
    </div>
  );
}

export function Empty({ title, children, actions }: { title: string; children?: ReactNode; actions?: ReactNode }) {
  return (
    <div className="rounded-panel border border-dashed border-line-strong px-4 py-10 text-center">
      <p className="text-heading">{title}</p>
      {children && <p className="mx-auto mt-1 max-w-[46ch] text-ink-2">{children}</p>}
      {actions && <div className="mt-4 flex flex-wrap justify-center gap-2">{actions}</div>}
    </div>
  );
}

// Placeholder bars in the shape of the content to come; they pulse only if motion is welcome.
export function Loading() {
  return (
    <div role="status" aria-busy="true" aria-label="Загрузка" className={`space-y-3 border-line p-4 sm:p-5 ${panelClass}`}>
      {["w-1/3", "w-4/5", "w-3/5"].map((width) => (
        <div key={width} className={`h-4 rounded-tag bg-sunken motion-safe:animate-pulse ${width}`} />
      ))}
    </div>
  );
}
