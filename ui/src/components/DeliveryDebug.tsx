import { useState } from "react";
import { api, type DeliveryDebugResponse } from "../api";
import { usePolling, timeAgo } from "../hooks";

export function DeliveryDebug() {
  const [expanded, setExpanded] = useState(false);
  const { data, error } = usePolling<DeliveryDebugResponse>(
    () => api.deliveryDebug(),
    5000
  );

  const receipts = data?.receipts;
  const batches = data?.recent_batches ?? [];

  return (
    <div className="border border-[var(--border)] rounded-lg overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)] flex items-center justify-between cursor-pointer hover:bg-[var(--bg-hover)] transition-colors"
      >
        <span className="text-[var(--text-secondary)] text-xs uppercase tracking-wider">
          Delivery Debug
          {receipts && (
            <span className="ml-2 normal-case tracking-normal text-[var(--text-muted)]">
              {receipts.pending > 0 && (
                <span className="text-[var(--yellow)]">
                  {receipts.pending} pending
                </span>
              )}
              {receipts.pending === 0 && receipts.total > 0 && (
                <span className="text-[var(--green)]">all delivered</span>
              )}
            </span>
          )}
        </span>
        <span className="text-[var(--text-muted)] text-xs">
          {expanded ? "▼" : "▶"}
        </span>
      </button>

      {expanded && (
        <div className="p-3 space-y-3">
          {error && (
            <div className="text-xs text-[var(--red)]">Error: {error}</div>
          )}

          {receipts && (
            <div>
              <div className="text-[var(--text-muted)] text-xs mb-1.5">
                Receipt Summary
              </div>
              <div className="grid grid-cols-4 gap-2 text-center">
                <div className="bg-[var(--bg-tertiary)] rounded p-2">
                  <div className="text-lg font-medium">{receipts.total}</div>
                  <div className="text-[var(--text-muted)] text-xs">total</div>
                </div>
                <div className="bg-[var(--bg-tertiary)] rounded p-2">
                  <div className="text-lg font-medium text-[var(--yellow)]">
                    {receipts.pending}
                  </div>
                  <div className="text-[var(--text-muted)] text-xs">
                    pending
                  </div>
                </div>
                <div className="bg-[var(--bg-tertiary)] rounded p-2">
                  <div className="text-lg font-medium text-[var(--text-secondary)]">
                    {receipts.offered}
                  </div>
                  <div className="text-[var(--text-muted)] text-xs">
                    offered
                  </div>
                </div>
                <div className="bg-[var(--bg-tertiary)] rounded p-2">
                  <div className="text-lg font-medium text-[var(--green)]">
                    {receipts.delivered}
                  </div>
                  <div className="text-[var(--text-muted)] text-xs">
                    delivered
                  </div>
                </div>
              </div>
            </div>
          )}

          {batches.length > 0 && (
            <div>
              <div className="text-[var(--text-muted)] text-xs mb-1.5">
                Recent Sync Batches ({batches.length})
              </div>
              <div className="border border-[var(--border)] rounded overflow-hidden">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="bg-[var(--bg-tertiary)]">
                      <th className="text-left px-2 py-1 text-[var(--text-muted)] font-normal">
                        Agent
                      </th>
                      <th className="text-left px-2 py-1 text-[var(--text-muted)] font-normal">
                        Token
                      </th>
                      <th className="text-left px-2 py-1 text-[var(--text-muted)] font-normal">
                        When
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--border)]">
                    {batches.map((b) => (
                      <tr key={b.token} className="hover:bg-[var(--bg-hover)]">
                        <td className="px-2 py-1 text-[var(--text-secondary)]">
                          {b.agent_name}
                        </td>
                        <td className="px-2 py-1">
                          <code className="text-[var(--text-muted)]">
                            {b.token.slice(0, 8)}...
                          </code>
                        </td>
                        <td className="px-2 py-1 text-[var(--text-muted)]">
                          {timeAgo(b.created_at)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
