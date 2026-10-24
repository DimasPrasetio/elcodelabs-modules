import { useEffect, useRef, useState } from "react";
import {
  SelectContent,
  SelectList,
  SelectMessage,
  SelectOptionButton,
  SelectRoot,
  SelectSearch,
  SelectTrigger,
} from "@/shared/components/ui/select-parts";
import type { SelectOption, SelectValue } from "@/shared/components/ui/select-parts";

export interface AsyncLoadResult {
  options: SelectOption[];
  /** More results exist than were returned. */
  hasMore?: boolean;
}

export interface AsyncSearchSelectProps {
  /**
   * Loads options for a search. Pass signal on to the HTTP call: it is aborted when a newer
   * search replaces this one or the select closes. page (from 1) is sent only when paged.
   */
  loadOptions: (search: string, signal: AbortSignal, page: number) => Promise<AsyncLoadResult | SelectOption[]>;
  value: SelectValue | null | undefined;
  onChange: (value: SelectValue | null, option?: SelectOption) => void;
  /** The chosen option's label when the list has not loaded it (e.g. an edit form). */
  selectedOption?: SelectOption | null;
  placeholder?: string;
  searchPlaceholder?: string;
  allowClear?: boolean;
  disabled?: boolean;
  id?: string;
  name?: string;
  debounceMs?: number;
  /** Do not search until this many characters are typed (an empty search still loads the first page). */
  minChars?: number;
  /** Load the next page when the list is scrolled near its end, while hasMore. */
  paged?: boolean;
  emptyText?: string;
  loadingText?: string;
  moreText?: string;
  "aria-label"?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
  className?: string;
}

function normalize(r: AsyncLoadResult | SelectOption[]): AsyncLoadResult {
  return Array.isArray(r) ? { options: r } : r;
}

const isAbort = (err: unknown) =>
  (err instanceof DOMException && err.name === "AbortError") || (err as { code?: string } | null)?.code === "ERR_CANCELED";

/**
 * A select whose options come from the server as the user types: debounced, stale requests
 * aborted, Enter waits for the latest results, optional infinite paging.
 */
export function AsyncSearchSelect({
  loadOptions,
  value,
  onChange,
  selectedOption,
  placeholder = "Pilih…",
  searchPlaceholder = "Cari…",
  allowClear = false,
  disabled,
  id,
  name,
  debounceMs = 250,
  minChars = 0,
  paged = false,
  emptyText = "Tidak ada hasil",
  loadingText = "Memuat…",
  moreText = "Ketik lebih spesifik untuk melihat hasil lainnya",
  className,
  ...aria
}: AsyncSearchSelectProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [options, setOptions] = useState<SelectOption[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);
  const [chosen, setChosen] = useState<SelectOption | null>(selectedOption ?? null);
  const abort = useRef<AbortController | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const sentinel = useRef<HTMLDivElement>(null);
  const tooShort = search.trim().length > 0 && search.trim().length < minChars;

  // An outside value or label change wins over what this select remembers.
  const [prevSelected, setPrevSelected] = useState(selectedOption);
  if (selectedOption !== prevSelected) {
    setPrevSelected(selectedOption);
    setChosen(selectedOption ?? null);
  }

  function run(q: string, nextPage: number) {
    abort.current?.abort();
    if (q.trim().length > 0 && q.trim().length < minChars) {
      setOptions([]);
      setLoading(false);
      return;
    }
    const controller = new AbortController();
    abort.current = controller;
    setLoading(true);
    setError(null);
    loadOptions(q.trim(), controller.signal, nextPage)
      .then((r) => {
        if (controller.signal.aborted) return;
        const res = normalize(r);
        setOptions((prev) => {
          if (nextPage === 1) return res.options;
          const seen = new Set(prev.map((o) => o.value));
          return [...prev, ...res.options.filter((o) => !seen.has(o.value))];
        });
        setHasMore(Boolean(res.hasMore));
        setPage(nextPage);
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted || isAbort(err)) return;
        setError(err instanceof Error ? err.message : "Gagal memuat pilihan");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
  }

  function schedule(q: string) {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      timer.current = null;
      run(q, 1);
    }, debounceMs);
  }

  useEffect(
    () => () => {
      abort.current?.abort();
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  // Next page when the end of the list scrolls into view.
  useEffect(() => {
    if (!paged || !open || !hasMore || loading || !sentinel.current) return;
    const io = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) run(search, page + 1);
    });
    io.observe(sentinel.current);
    return () => io.disconnect();
    // run reads the latest search and page from this render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paged, open, hasMore, loading, page, search]);

  function onOpenChange(next: boolean) {
    if (disabled) return;
    setOpen(next);
    if (next) {
      setSearch("");
      run("", 1);
    } else {
      abort.current?.abort();
    }
  }

  function choose(o: SelectOption) {
    setChosen(o);
    onChange(o.value, o);
    setOpen(false);
  }

  const hasValue = value !== null && value !== undefined && value !== "";
  const label = hasValue ? (chosen && chosen.value === value ? chosen.label : options.find((o) => o.value === value)?.label ?? String(value)) : placeholder;

  return (
    <SelectRoot open={open && !disabled} onOpenChange={onOpenChange}>
      {name && <input type="hidden" name={name} value={hasValue ? String(value) : ""} />}
      <SelectTrigger
        id={id}
        disabled={disabled}
        label={label}
        empty={!hasValue}
        open={open}
        onClear={
          allowClear && hasValue && !disabled
            ? () => {
                setChosen(null);
                onChange(null);
              }
            : undefined
        }
        className={className}
        {...aria}
      />
      <SelectContent>
        <SelectSearch
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            schedule(e.target.value);
          }}
          onKeyDown={(e) => {
            // Results are stale while a search is pending: do not pick from them.
            if (e.key === "Enter" && (loading || timer.current !== null)) {
              if (timer.current) {
                clearTimeout(timer.current);
                timer.current = null;
                run(search, 1);
              }
              e.preventDefault();
            }
          }}
          placeholder={searchPlaceholder}
          aria-label={searchPlaceholder}
        />
        <SelectList busy={loading}>
          {options.map((o) => (
            <SelectOptionButton key={o.value} option={o} active={o.value === value} onChoose={() => choose(o)} />
          ))}
          {loading && <SelectMessage role="status">{loadingText}</SelectMessage>}
          {!loading && error && <SelectMessage role="alert">{error}</SelectMessage>}
          {tooShort && <SelectMessage>Ketik minimal {minChars} huruf untuk mencari</SelectMessage>}
          {!loading && !error && !tooShort && options.length === 0 && <SelectMessage>{emptyText}</SelectMessage>}
          {!loading && !error && hasMore && !paged && <SelectMessage>{moreText}</SelectMessage>}
          {paged && hasMore && <div ref={sentinel} aria-hidden className="h-px shrink-0" />}
        </SelectList>
      </SelectContent>
    </SelectRoot>
  );
}
