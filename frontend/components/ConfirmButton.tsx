"use client";

import { useState } from "react";
import { Button } from "@/components/ui";

export default function ConfirmButton({
  label,
  confirmLabel,
  onConfirm,
  disabled,
}: {
  label: string;
  confirmLabel: string;
  onConfirm: () => void;
  disabled?: boolean;
}) {
  const [asking, setAsking] = useState(false);
  if (!asking) {
    return (
      <Button type="button" variant="danger" disabled={disabled} onClick={() => setAsking(true)}>
        {label}
      </Button>
    );
  }
  return (
    <span className="inline-flex flex-wrap gap-2">
      <Button
        type="button"
        variant="dangerSolid"
        disabled={disabled}
        onClick={() => {
          setAsking(false);
          onConfirm();
        }}
      >
        {confirmLabel}
      </Button>
      <Button type="button" variant="ghost" autoFocus onClick={() => setAsking(false)}>
        Отмена
      </Button>
    </span>
  );
}
