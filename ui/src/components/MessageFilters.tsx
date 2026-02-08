import type { MessageFilters as Filters } from "../api";

interface Props {
  filters: Filters;
  onChange: (filters: Filters) => void;
  agentNames: string[];
}

export function MessageFilters({ filters, onChange, agentNames }: Props) {
  const set = (key: keyof Filters, value: string) => {
    onChange({ ...filters, [key]: value || undefined, offset: 0 });
  };

  const clear = () => {
    onChange({ limit: filters.limit });
  };

  const hasFilters =
    filters.q || filters.from || filters.to || filters.kind || filters.state;

  return (
    <div className="flex flex-wrap items-center gap-2">
      <input
        type="text"
        placeholder="Search messages..."
        value={filters.q || ""}
        onChange={(e) => set("q", e.target.value)}
        className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent)] w-full sm:w-48"
      />

      <select
        value={filters.from || ""}
        onChange={(e) => set("from", e.target.value)}
        className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
      >
        <option value="">From: all</option>
        {agentNames.map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>

      <select
        value={filters.to || ""}
        onChange={(e) => set("to", e.target.value)}
        className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
      >
        <option value="">To: all</option>
        {agentNames.map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>

      <select
        value={filters.kind || ""}
        onChange={(e) => set("kind", e.target.value)}
        className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
      >
        <option value="">Kind: all</option>
        <option value="direct">direct</option>
        <option value="broadcast">broadcast</option>
      </select>

      <select
        value={filters.state || ""}
        onChange={(e) => set("state", e.target.value)}
        className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
      >
        <option value="">State: all</option>
        <option value="pending">pending</option>
        <option value="offered">offered</option>
        <option value="delivered">delivered</option>
      </select>

      {hasFilters && (
        <button
          onClick={clear}
          className="px-2 py-1 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
        >
          clear
        </button>
      )}
    </div>
  );
}
