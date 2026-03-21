import { useEffect, useState } from "react";

/**
 * useDebounce returns a debounced version of the input value.
 * The returned value only updates after the specified delay has
 * elapsed since the last change to the input value.
 */
export function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}
