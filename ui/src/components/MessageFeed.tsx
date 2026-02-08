import type { Message, PageInfo } from "../api";
import { timeAgo } from "../hooks";

interface Props {
  messages: Message[];
  page: PageInfo;
  onPageChange: (offset: number) => void;
  onSelect: (id: string) => void;
}

function DeliveryBadge({ delivery }: { delivery: Message["delivery"] }) {
  if (delivery.total === 0) return null;

  if (delivery.delivered === delivery.total) {
    return (
      <span className="text-[var(--green)] text-xs">
        delivered
      </span>
    );
  }
  if (delivery.pending === delivery.total) {
    return (
      <span className="text-[var(--yellow)] text-xs">
        pending
      </span>
    );
  }
  return (
    <span className="text-[var(--text-muted)] text-xs">
      {delivery.delivered}/{delivery.total}
    </span>
  );
}

export function MessageFeed({ messages, page, onPageChange, onSelect }: Props) {
  const totalPages = Math.ceil(page.total / page.limit);
  const currentPage = Math.floor(page.offset / page.limit) + 1;

  return (
    <div className="border border-[var(--border)] rounded-lg overflow-hidden">
      <div className="px-3 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)] flex items-center justify-between">
        <span className="text-[var(--text-secondary)] text-xs uppercase tracking-wider">
          Messages ({page.total})
        </span>
        {totalPages > 1 && (
          <div className="flex items-center gap-2 text-xs">
            <button
              disabled={currentPage <= 1}
              onClick={() => onPageChange(page.offset - page.limit)}
              className="px-1.5 py-0.5 text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-30 disabled:cursor-default cursor-pointer"
            >
              prev
            </button>
            <span className="text-[var(--text-muted)]">
              {currentPage}/{totalPages}
            </span>
            <button
              disabled={currentPage >= totalPages}
              onClick={() => onPageChange(page.offset + page.limit)}
              className="px-1.5 py-0.5 text-[var(--text-secondary)] hover:text-[var(--text-primary)] disabled:opacity-30 disabled:cursor-default cursor-pointer"
            >
              next
            </button>
          </div>
        )}
      </div>

      <div className="divide-y divide-[var(--border)]">
        {messages.length === 0 && (
          <div className="px-3 py-8 text-center text-[var(--text-muted)]">
            No messages
          </div>
        )}
        {messages.map((m) => (
          <div
            key={m.id}
            onClick={() => onSelect(m.id)}
            className="px-3 py-2 hover:bg-[var(--bg-hover)] cursor-pointer transition-colors"
          >
            {/* Desktop: single row */}
            <div className="hidden sm:flex items-center gap-2">
              <span className="text-[var(--text-muted)] text-xs w-16 shrink-0">
                {timeAgo(m.created_at)}
              </span>
              <span className="font-medium text-[var(--accent)]">
                {m.from}
              </span>
              <span className="text-[var(--text-muted)]">→</span>
              <span className="text-[var(--text-secondary)]">
                {m.recipients.join(", ") || "(broadcast)"}
              </span>
              <span
                className={`px-1.5 py-0.5 rounded text-xs ${
                  m.kind === "broadcast"
                    ? "bg-purple-900/30 text-purple-400"
                    : "bg-blue-900/30 text-blue-400"
                }`}
              >
                {m.kind}
              </span>
              <DeliveryBadge delivery={m.delivery} />
            </div>
            {/* Mobile: stacked */}
            <div className="sm:hidden space-y-1">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <span className="font-medium text-[var(--accent)] text-sm">
                    {m.from}
                  </span>
                  <span className="text-[var(--text-muted)]">→</span>
                  <span className="text-[var(--text-secondary)] text-sm truncate max-w-[120px]">
                    {m.recipients.join(", ") || "(broadcast)"}
                  </span>
                </div>
                <span className="text-[var(--text-muted)] text-xs shrink-0">
                  {timeAgo(m.created_at)}
                </span>
              </div>
              <div className="flex items-center gap-1.5">
                <span
                  className={`px-1.5 py-0.5 rounded text-xs ${
                    m.kind === "broadcast"
                      ? "bg-purple-900/30 text-purple-400"
                      : "bg-blue-900/30 text-blue-400"
                  }`}
                >
                  {m.kind}
                </span>
                <DeliveryBadge delivery={m.delivery} />
              </div>
            </div>
            <div className="text-[var(--text-secondary)] text-xs mt-1 sm:ml-18 truncate">
              {m.body}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
