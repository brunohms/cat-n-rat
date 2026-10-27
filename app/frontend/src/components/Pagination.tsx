export const PAGE_SIZES = [25, 50, 100, 200];

interface Props {
  total: number;
  limit: number;
  offset: number;
  onChange: (offset: number, limit: number) => void;
}

export function Pagination({ total, limit, offset, onChange }: Props) {
  const page = Math.floor(offset / limit) + 1;
  const pages = Math.max(1, Math.ceil(total / limit));
  const first = total === 0 ? 0 : offset + 1;
  const last = Math.min(offset + limit, total);

  return (
    <nav className="pagination" aria-label="Pagination">
      <span className="muted">
        {first}–{last} of {total}
      </span>
      <div className="pagination-controls">
        <label className="page-size">
          Rows
          <select value={limit} onChange={(event) => onChange(0, Number(event.target.value))}>
            {PAGE_SIZES.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </label>
        <button type="button" onClick={() => onChange(0, limit)} disabled={page <= 1} aria-label="First page">
          «
        </button>
        <button type="button" onClick={() => onChange(offset - limit, limit)} disabled={page <= 1} aria-label="Previous page">
          ‹
        </button>
        <span>
          Page {page} of {pages}
        </span>
        <button type="button" onClick={() => onChange(offset + limit, limit)} disabled={page >= pages} aria-label="Next page">
          ›
        </button>
        <button
          type="button"
          onClick={() => onChange((pages - 1) * limit, limit)}
          disabled={page >= pages}
          aria-label="Last page"
        >
          »
        </button>
      </div>
    </nav>
  );
}
