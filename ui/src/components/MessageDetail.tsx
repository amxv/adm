import { useEffect, useState } from "react";
import { api, type MessageDetail as MessageDetailType } from "../api";

interface Props {
  messageId: string;
  onClose: () => void;
}

export function MessageDetail({ messageId, onClose }: Props) {
  const [msg, setMsg] = useState<MessageDetailType | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .message(messageId)
      .then(setMsg)
      .catch((e) => setError(e.message));
  }, [messageId]);

  if (error) {
    return (
      <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4 max-w-lg w-full mx-4">
          <div className="text-[var(--red)]">Error: {error}</div>
          <button
            onClick={onClose}
            className="mt-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            close
          </button>
        </div>
      </div>
    );
  }

  if (!msg) {
    return (
      <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
        <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg p-4">
          <span className="text-[var(--text-muted)]">Loading...</span>
        </div>
      </div>
    );
  }

  return (
    <div
      className="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg max-w-2xl w-full mx-4 max-h-[80vh] overflow-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-[var(--border)] flex items-center justify-between">
          <span className="text-xs text-[var(--text-muted)]">
            Message Detail
          </span>
          <button
            onClick={onClose}
            className="text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer text-sm"
          >
            esc
          </button>
        </div>

        <div className="p-4 space-y-3">
          <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs">
            <span className="text-[var(--text-muted)]">ID</span>
            <code className="text-[var(--text-secondary)]">{msg.id}</code>

            <span className="text-[var(--text-muted)]">From</span>
            <span className="text-[var(--accent)]">{msg.from}</span>

            <span className="text-[var(--text-muted)]">Kind</span>
            <span
              className={`inline-block w-fit px-1.5 py-0.5 rounded ${
                msg.kind === "broadcast"
                  ? "bg-purple-900/30 text-purple-400"
                  : "bg-blue-900/30 text-blue-400"
              }`}
            >
              {msg.kind}
            </span>

            <span className="text-[var(--text-muted)]">Created</span>
            <span>{new Date(msg.created_at).toLocaleString()}</span>
          </div>

          <div>
            <div className="text-[var(--text-muted)] text-xs mb-1">Body</div>
            <div className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded p-3 text-sm whitespace-pre-wrap">
              {msg.body}
            </div>
          </div>

          <div>
            <div className="text-[var(--text-muted)] text-xs mb-1">
              Receipts ({msg.receipts.length})
            </div>
            <div className="border border-[var(--border)] rounded overflow-hidden">
              <table className="w-full text-xs">
                <thead>
                  <tr className="bg-[var(--bg-tertiary)]">
                    <th className="text-left px-3 py-1.5 text-[var(--text-muted)] font-normal">
                      Recipient
                    </th>
                    <th className="text-left px-3 py-1.5 text-[var(--text-muted)] font-normal">
                      State
                    </th>
                    <th className="text-left px-3 py-1.5 text-[var(--text-muted)] font-normal">
                      Offered
                    </th>
                    <th className="text-left px-3 py-1.5 text-[var(--text-muted)] font-normal">
                      Delivered
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--border)]">
                  {msg.receipts.map((r) => (
                    <tr key={r.recipient} className="hover:bg-[var(--bg-hover)]">
                      <td className="px-3 py-1.5">{r.recipient}</td>
                      <td className="px-3 py-1.5">
                        <span
                          className={
                            r.state === "delivered"
                              ? "text-[var(--green)]"
                              : r.state === "pending"
                              ? "text-[var(--yellow)]"
                              : "text-[var(--text-secondary)]"
                          }
                        >
                          {r.state}
                        </span>
                      </td>
                      <td className="px-3 py-1.5 text-[var(--text-muted)]">
                        {r.offered_at
                          ? new Date(r.offered_at).toLocaleString()
                          : "—"}
                      </td>
                      <td className="px-3 py-1.5 text-[var(--text-muted)]">
                        {r.delivered_at
                          ? new Date(r.delivered_at).toLocaleString()
                          : "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
