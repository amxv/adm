import { api, type ConflictsResponse } from "../api";
import { usePolling } from "../hooks";

export function ConflictRadar() {
  const { data, error } = usePolling<ConflictsResponse>(
    () => api.claimConflicts(),
    10000
  );

  if (error) {
    return (
      <div className="border border-[var(--border)] rounded-lg overflow-hidden">
        <div className="px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
          <span className="text-[var(--text-secondary)] text-xs uppercase tracking-wider">
            Conflicts
          </span>
        </div>
        <div className="px-3 py-3 text-xs text-[var(--red)]">
          Error: {error}
        </div>
      </div>
    );
  }

  const conflicts = data?.conflicts ?? [];

  return (
    <div className="border border-[var(--border)] rounded-lg overflow-hidden">
      <div className="px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <span className="text-[var(--text-secondary)] text-xs uppercase tracking-wider">
          Conflicts{" "}
          {conflicts.length > 0 && (
            <span className="text-[var(--red)]">({conflicts.length})</span>
          )}
          {conflicts.length === 0 && (
            <span className="text-[var(--text-muted)]">(0)</span>
          )}
        </span>
      </div>
      <div className="divide-y divide-[var(--border)]">
        {conflicts.length === 0 && (
          <div className="px-3 py-3 text-center text-[var(--text-muted)] text-xs">
            No overlapping claims
          </div>
        )}
        {conflicts.map((c, i) => (
          <div key={i} className="px-3 py-2">
            <div className="flex items-center gap-1 text-xs">
              <span className="text-[var(--red)]">{c.overlap_type}</span>
            </div>
            <div className="mt-1 ml-1 space-y-0.5">
              <div className="text-xs">
                <span className="text-[var(--text-secondary)]">
                  {c.claim_a.agent_name}
                </span>
                <code className="ml-1 text-[var(--accent)]">
                  {c.claim_a.path_pattern}
                </code>
              </div>
              <div className="text-xs">
                <span className="text-[var(--text-secondary)]">
                  {c.claim_b.agent_name}
                </span>
                <code className="ml-1 text-[var(--accent)]">
                  {c.claim_b.path_pattern}
                </code>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
