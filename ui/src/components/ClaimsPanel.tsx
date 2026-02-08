import type { Claim } from "../api";
import { timeAgo } from "../hooks";

interface Props {
  claims: Claim[];
}

export function ClaimsPanel({ claims }: Props) {
  // Group claims by agent.
  const grouped = new Map<string, Claim[]>();
  for (const c of claims) {
    const list = grouped.get(c.agent_name) || [];
    list.push(c);
    grouped.set(c.agent_name, list);
  }

  return (
    <div className="border border-[var(--border)] rounded-lg overflow-hidden">
      <div className="px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <span className="text-[var(--text-secondary)] text-xs uppercase tracking-wider">
          Claims ({claims.length})
        </span>
      </div>
      <div className="divide-y divide-[var(--border)]">
        {claims.length === 0 && (
          <div className="px-3 py-4 text-center text-[var(--text-muted)]">
            No active claims
          </div>
        )}
        {[...grouped.entries()].map(([agent, agentClaims]) => (
          <div key={agent} className="px-3 py-2">
            <div className="font-medium text-xs text-[var(--text-secondary)] mb-1">
              {agent}
            </div>
            {agentClaims.map((c) => (
              <div
                key={c.path_norm}
                className="flex items-center justify-between py-0.5 ml-2"
              >
                <code className="text-xs text-[var(--accent)]">
                  {c.path_pattern}
                </code>
                <span className="text-[var(--text-muted)] text-xs">
                  {timeAgo(c.updated_at)}
                </span>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
