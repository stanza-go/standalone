import { useCallback, useState } from "react";

interface UseTableKeyboardOptions {
  rowCount: number;
  onActivate?: (index: number) => void;
  onSelect?: (index: number) => void;
}

export function useTableKeyboard({ rowCount, onActivate, onSelect }: UseTableKeyboardOptions) {
  const [focusedRow, setFocusedRow] = useState(-1);

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (rowCount === 0) return;

      switch (e.key) {
        case "ArrowDown": {
          e.preventDefault();
          setFocusedRow((prev) => (prev < rowCount - 1 ? prev + 1 : prev));
          break;
        }
        case "ArrowUp": {
          e.preventDefault();
          setFocusedRow((prev) => (prev > 0 ? prev - 1 : prev));
          break;
        }
        case "Home": {
          e.preventDefault();
          setFocusedRow(0);
          break;
        }
        case "End": {
          e.preventDefault();
          setFocusedRow(rowCount - 1);
          break;
        }
        case "Enter": {
          if (focusedRow >= 0 && onActivate) {
            e.preventDefault();
            onActivate(focusedRow);
          }
          break;
        }
        case " ": {
          if (focusedRow >= 0 && onSelect) {
            e.preventDefault();
            onSelect(focusedRow);
          }
          break;
        }
        case "Escape": {
          setFocusedRow(-1);
          break;
        }
      }
    },
    [rowCount, focusedRow, onActivate, onSelect],
  );

  const resetFocus = useCallback(() => setFocusedRow(-1), []);

  const tbodyProps = {
    tabIndex: 0,
    role: "grid" as const,
    onKeyDown,
    onBlur: resetFocus,
    style: { outline: "none" } as React.CSSProperties,
  };

  const isFocused = useCallback((index: number) => focusedRow === index, [focusedRow]);

  return { focusedRow, isFocused, tbodyProps, resetFocus };
}
