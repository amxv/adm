import { useCallback, useState } from "react";
import { api, type MessageFilters } from "./api";
import { usePolling } from "./hooks";
import { AgentPanel } from "./components/AgentPanel";
import { ClaimsPanel } from "./components/ClaimsPanel";
import { ConflictRadar } from "./components/ConflictRadar";
import { DeliveryDebug } from "./components/DeliveryDebug";
import { FilterPresets } from "./components/FilterPresets";
import { MessageFeed } from "./components/MessageFeed";
import { MessageFilters as MessageFiltersUI } from "./components/MessageFilters";
import { MessageDetail } from "./components/MessageDetail";

function App() {
  const [filters, setFilters] = useState<MessageFilters>({ limit: 50 });
  const [selectedMessage, setSelectedMessage] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);

  const messagesFetcher = useCallback(
    () => api.messages(filters),
    [filters]
  );

  const agents = usePolling(() => api.agents(), 5000);
  const claims = usePolling(() => api.claims(), 10000);
  const messages = usePolling(messagesFetcher, 3000);

  const agentNames = (agents.data || []).map((a) => a.name);

  const setFilterFrom = (name: string) => {
    setFilters((f) => ({ ...f, from: name, offset: 0 }));
  };

  const setFilterTo = (name: string) => {
    setFilters((f) => ({ ...f, to: name, offset: 0 }));
  };

  const handlePageChange = (offset: number) => {
    setFilters((f) => ({ ...f, offset: Math.max(0, offset) }));
  };

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <div className="max-w-7xl mx-auto px-4 py-2 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="md:hidden text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer text-sm"
            >
              {sidebarOpen ? "✕" : "☰"}
            </button>
            <span className="font-bold text-sm tracking-tight">ADM</span>
            <span className="text-[var(--text-muted)] text-xs hidden sm:inline">
              Agent DM Dashboard
            </span>
          </div>
          {agents.error && (
            <span className="text-[var(--red)] text-xs">
              API error: {agents.error}
            </span>
          )}
        </div>
      </header>

      {/* Main Layout */}
      <div className="max-w-7xl mx-auto px-4 py-4">
        <div className="grid grid-cols-1 md:grid-cols-[280px_1fr] gap-4">
          {/* Sidebar */}
          <div
            className={`space-y-4 ${
              sidebarOpen ? "block" : "hidden"
            } md:block`}
          >
            <AgentPanel
              agents={agents.data || []}
              onFilterFrom={setFilterFrom}
              onFilterTo={setFilterTo}
            />
            <ClaimsPanel claims={claims.data || []} />
            <ConflictRadar />
          </div>

          {/* Main Content */}
          <div className="space-y-3">
            <FilterPresets filters={filters} onApply={setFilters} />

            <MessageFiltersUI
              filters={filters}
              onChange={setFilters}
              agentNames={agentNames}
            />

            {messages.loading && !messages.data ? (
              <div className="text-center py-8 text-[var(--text-muted)]">
                Loading...
              </div>
            ) : messages.error ? (
              <div className="text-center py-8 text-[var(--red)]">
                Error: {messages.error}
              </div>
            ) : messages.data ? (
              <MessageFeed
                messages={messages.data.items}
                page={messages.data.page}
                onPageChange={handlePageChange}
                onSelect={setSelectedMessage}
              />
            ) : null}

            <DeliveryDebug />
          </div>
        </div>
      </div>

      {/* Message Detail Modal */}
      {selectedMessage && (
        <MessageDetail
          messageId={selectedMessage}
          onClose={() => setSelectedMessage(null)}
        />
      )}
    </div>
  );
}

export default App;
