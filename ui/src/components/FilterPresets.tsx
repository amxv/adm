import { useState } from "react";
import type { MessageFilters } from "../api";
import {
  builtinPresets,
  loadCustomPresets,
  addCustomPreset,
  deleteCustomPreset,
  type FilterPreset,
} from "../presets";

interface Props {
  filters: MessageFilters;
  onApply: (filters: MessageFilters) => void;
}

export function FilterPresets({ filters, onApply }: Props) {
  const [customPresets, setCustomPresets] = useState<FilterPreset[]>(
    loadCustomPresets
  );
  const [saving, setSaving] = useState(false);
  const [newName, setNewName] = useState("");

  const allPresets = [...builtinPresets, ...customPresets];

  const handleSave = () => {
    const name = newName.trim();
    if (!name) return;
    const updated = addCustomPreset(name, { ...filters });
    setCustomPresets(updated);
    setNewName("");
    setSaving(false);
  };

  const handleDelete = (name: string) => {
    const updated = deleteCustomPreset(name);
    setCustomPresets(updated);
  };

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {allPresets.map((p) => (
        <div key={p.name} className="flex items-center">
          <button
            onClick={() => onApply({ ...p.filters })}
            className="px-2 py-0.5 text-xs rounded border border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--accent)] cursor-pointer transition-colors"
          >
            {p.name}
          </button>
          {!p.builtin && (
            <button
              onClick={() => handleDelete(p.name)}
              className="ml-0.5 px-1 text-xs text-[var(--text-muted)] hover:text-[var(--red)] cursor-pointer"
              title="Delete preset"
            >
              x
            </button>
          )}
        </div>
      ))}

      {saving ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            handleSave();
          }}
          className="flex items-center gap-1"
        >
          <input
            autoFocus
            type="text"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Preset name"
            className="bg-[var(--bg-tertiary)] border border-[var(--border)] rounded px-1.5 py-0.5 text-xs text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none focus:border-[var(--accent)] w-28"
          />
          <button
            type="submit"
            className="px-1.5 py-0.5 text-xs text-[var(--green)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            save
          </button>
          <button
            type="button"
            onClick={() => setSaving(false)}
            className="px-1 py-0.5 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            cancel
          </button>
        </form>
      ) : (
        <button
          onClick={() => setSaving(true)}
          className="px-2 py-0.5 text-xs rounded border border-dashed border-[var(--border)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:border-[var(--accent)] cursor-pointer transition-colors"
        >
          + save filter
        </button>
      )}
    </div>
  );
}
