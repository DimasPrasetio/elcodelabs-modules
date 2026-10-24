import { useId } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/shared/components/primitives/button";
import { Input } from "@/shared/components/primitives/input";

export interface KeyValueRow {
  key: string;
  value: string;
}

export interface KeyValueEditorProps {
  rows: KeyValueRow[];
  onChange: (rows: KeyValueRow[]) => void;
  maxRows?: number;
  maxKey?: number;
  maxValue?: number;
  /** Name suggestions (the user may still type any name). */
  suggestions?: readonly string[];
  /** An error for the whole block, e.g. from the API. */
  error?: string;
  disabled?: boolean;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  addLabel?: string;
  emptyText?: string;
}

/** Free name/value pairs ("Info tambahan"): empty and duplicate names are flagged per row. */
export function KeyValueEditor({
  rows,
  onChange,
  maxRows = 20,
  maxKey = 60,
  maxValue = 255,
  suggestions,
  error,
  disabled,
  keyPlaceholder = "Nama (mis. NPWP)",
  valuePlaceholder = "Isi",
  addLabel = "Tambah info",
  emptyText = "Belum ada informasi tambahan.",
}: KeyValueEditorProps) {
  const listId = useId();
  const counts = new Map<string, number>();
  for (const r of rows) {
    const k = r.key.trim().toLowerCase();
    if (k) counts.set(k, (counts.get(k) ?? 0) + 1);
  }
  const rowError = (r: KeyValueRow) => {
    const k = r.key.trim();
    if (!k && r.value.trim()) return "Nama wajib diisi";
    if (k && (counts.get(k.toLowerCase()) ?? 0) > 1) return "Nama sudah dipakai";
    return undefined;
  };
  const update = (i: number, patch: Partial<KeyValueRow>) => onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));

  return (
    <div className="space-y-3">
      {rows.length === 0 && <p className="text-sm text-muted-foreground">{emptyText}</p>}
      {suggestions && suggestions.length > 0 && (
        <datalist id={listId}>
          {suggestions.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
      )}
      {rows.map((r, i) => {
        const err = rowError(r);
        return (
          <div key={i} className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_2fr_auto]">
            <div>
              <Input
                aria-label={`Nama baris ${i + 1}`}
                aria-invalid={err ? true : undefined}
                list={suggestions?.length ? listId : undefined}
                value={r.key}
                maxLength={maxKey}
                placeholder={keyPlaceholder}
                disabled={disabled}
                onChange={(e) => update(i, { key: e.target.value })}
              />
              {err && <p className="mt-1 text-xs font-medium text-destructive">{err}</p>}
            </div>
            <Input
              aria-label={`Isi baris ${i + 1}`}
              value={r.value}
              maxLength={maxValue}
              placeholder={valuePlaceholder}
              disabled={disabled}
              onChange={(e) => update(i, { value: e.target.value })}
            />
            <Button variant="ghost" size="sm" disabled={disabled} onClick={() => onChange(rows.filter((_, j) => j !== i))}>
              Hapus
            </Button>
          </div>
        );
      })}
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button variant="outline" size="sm" disabled={disabled || rows.length >= maxRows} onClick={() => onChange([...rows, { key: "", value: "" }])}>
        <Plus aria-hidden />
        {addLabel}
      </Button>
    </div>
  );
}
