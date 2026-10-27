import { useCallback, useEffect, useRef, useState } from "react";

export type ParamChanges = Record<string, string | number | null | undefined>;

/**
 * Keeps state in the URL query string, so a filtered view can be shared and the back button works.
 * Empty values remove the parameter.
 */
export function useUrlParams() {
  const [params, setParams] = useState(() => new URLSearchParams(window.location.search));
  const current = useRef(params);

  useEffect(() => {
    const onPopState = () => {
      current.current = new URLSearchParams(window.location.search);
      setParams(current.current);
    };
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  const update = useCallback((changes: ParamChanges, { reset = false } = {}) => {
    const next = new URLSearchParams(reset ? "" : current.current);
    for (const [key, value] of Object.entries(changes)) {
      if (value === undefined || value === null || value === "") next.delete(key);
      else next.set(key, String(value));
    }
    if (next.toString() === current.current.toString()) return;

    const query = next.toString();
    window.history.pushState(null, "", query ? `?${query}` : window.location.pathname);
    current.current = next;
    setParams(next);
  }, []);

  return [params, update] as const;
}
