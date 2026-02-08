import type { Agent } from "../api";
import { timeAgo } from "../hooks";

interface Props {
  agents: Agent[];
  onFilterFrom: (name: string) => void;
  onFilterTo: (name: string) => void;
}

export function AgentPanel({ agents, onFilterFrom, onFilterTo }: Props) {
  return (
    <div className="border border-[var(--border)] rounded-lg overflow-hidden">
      <div className="px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <span className="text-[var(--text-secondary)] text-xs uppercase tracking-wider">
          Agents ({agents.length})
        </span>
      </div>
      <div className="divide-y divide-[var(--border)]">
        {agents.length === 0 && (
          <div className="px-3 py-4 text-center text-[var(--text-muted)]">
            No agents registered
          </div>
        )}
        {agents.map((a) => (
          <div
            key={a.name}
            className="px-3 py-2 hover:bg-[var(--bg-hover)] transition-colors"
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span
                  className={`inline-block w-2 h-2 rounded-full ${
                    a.status === "online"
                      ? "bg-[var(--green)]"
                      : "bg-[var(--text-muted)]"
                  }`}
                />
                <span className="font-medium">{a.name}</span>
                <span className="text-[var(--text-muted)] text-xs">
                  [{a.status}]
                </span>
              </div>
              <div className="flex gap-1">
                <button
                  onClick={() => onFilterFrom(a.name)}
                  className="px-1.5 py-0.5 text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] hover:bg-[var(--bg-tertiary)] rounded cursor-pointer"
                  title={`Filter messages from ${a.name}`}
                >
                  from
                </button>
                <button
                  onClick={() => onFilterTo(a.name)}
                  className="px-1.5 py-0.5 text-xs text-[var(--text-secondary)] hover:text-[var(--accent)] hover:bg-[var(--bg-tertiary)] rounded cursor-pointer"
                  title={`Filter messages to ${a.name}`}
                >
                  to
                </button>
              </div>
            </div>
            {a.task && (
              <div className="text-[var(--text-secondary)] text-xs mt-1 ml-4 truncate">
                {a.task}
              </div>
            )}
            <div className="text-[var(--text-muted)] text-xs mt-0.5 ml-4">
              {timeAgo(a.last_seen_at)}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
