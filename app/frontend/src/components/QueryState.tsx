import type { ReactNode } from "react";

interface Props {
  isLoading: boolean;
  error: Error | null;
  isEmpty: boolean;
  colSpan: number;
  children: ReactNode;
}

/** Table body that shows loading, error and empty states in place of the rows. */
export function QueryState({ isLoading, error, isEmpty, colSpan, children }: Props) {
  let message: ReactNode = null;
  if (error) message = <span className="error">Failed to load data: {error.message}</span>;
  else if (isLoading) message = "Loading…";
  else if (isEmpty) message = "No results for these filters.";

  if (message) {
    return (
      <tbody>
        <tr>
          <td colSpan={colSpan} className="state">
            {message}
          </td>
        </tr>
      </tbody>
    );
  }
  return <>{children}</>;
}
