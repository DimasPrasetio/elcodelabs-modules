import { useState } from "react";
import type { ComponentProps } from "react";
import { Eye, EyeOff } from "lucide-react";
import { cn } from "cn";
import { FIELD } from "@/shared/lib/field-classes";

/** A password input with a show/hide button, in the standard field style. */
export function PasswordInput({ className, ...props }: Omit<ComponentProps<"input">, "type">) {
  const [visible, setVisible] = useState(false);
  const label = visible ? "Sembunyikan kata sandi" : "Tampilkan kata sandi";
  const Icon = visible ? EyeOff : Eye;
  return (
    <div className="relative">
      <input {...props} type={visible ? "text" : "password"} className={cn(FIELD, "pr-11", className)} />
      <button
        type="button"
        aria-label={label}
        aria-pressed={visible}
        title={label}
        onClick={() => setVisible((v) => !v)}
        className="absolute inset-y-0 right-0 flex items-center rounded-r-md px-3 text-muted-foreground transition-colors hover:text-foreground"
      >
        <Icon size={18} aria-hidden />
      </button>
    </div>
  );
}
