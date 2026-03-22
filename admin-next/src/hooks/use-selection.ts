import { useCallback, useState } from "react";

export function useSelection() {
  const [selected, setSelected] = useState<Set<number>>(new Set());

  const toggle = useCallback((id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const toggleAll = useCallback((ids: number[]) => {
    setSelected((prev) => {
      const allSelected = ids.length > 0 && ids.every((id) => prev.has(id));
      if (allSelected) {
        return new Set();
      }
      return new Set(ids);
    });
  }, []);

  const clear = useCallback(() => setSelected(new Set()), []);

  const isSelected = useCallback((id: number) => selected.has(id), [selected]);

  const isAllSelected = useCallback(
    (ids: number[]) => ids.length > 0 && ids.every((id) => selected.has(id)),
    [selected],
  );

  return {
    selected,
    count: selected.size,
    ids: Array.from(selected),
    toggle,
    toggleAll,
    clear,
    isSelected,
    isAllSelected,
  };
}
