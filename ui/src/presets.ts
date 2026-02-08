import type { MessageFilters } from "./api";

export interface FilterPreset {
  name: string;
  filters: MessageFilters;
  builtin?: boolean;
}

const STORAGE_KEY = "adm-filter-presets";

export const builtinPresets: FilterPreset[] = [
  { name: "All", filters: { limit: 50 }, builtin: true },
  { name: "Pending", filters: { state: "pending", limit: 50 }, builtin: true },
  {
    name: "Broadcasts",
    filters: { kind: "broadcast", limit: 50 },
    builtin: true,
  },
];

export function loadCustomPresets(): FilterPreset[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    return JSON.parse(raw) as FilterPreset[];
  } catch {
    return [];
  }
}

export function saveCustomPresets(presets: FilterPreset[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(presets));
}

export function addCustomPreset(name: string, filters: MessageFilters): FilterPreset[] {
  const presets = loadCustomPresets();
  presets.push({ name, filters });
  saveCustomPresets(presets);
  return presets;
}

export function deleteCustomPreset(name: string): FilterPreset[] {
  const presets = loadCustomPresets().filter((p) => p.name !== name);
  saveCustomPresets(presets);
  return presets;
}
