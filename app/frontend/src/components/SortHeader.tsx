import type { SortOrder } from "../api";

interface Props {
  label: string;
  field: string;
  sort: string;
  order: SortOrder;
  onSort: (field: string, order: SortOrder) => void;
  /** Order used when the column is first selected */
  defaultOrder?: SortOrder;
  align?: "left" | "right";
}

export function SortHeader({ label, field, sort, order, onSort, defaultOrder = "asc", align = "left" }: Props) {
  const active = sort === field;
  const nextOrder: SortOrder = active ? (order === "asc" ? "desc" : "asc") : defaultOrder;
  return (
    <th className={align === "right" ? "num" : undefined} aria-sort={active ? (order === "asc" ? "ascending" : "descending") : "none"}>
      <button type="button" className={`sort ${active ? "active" : ""}`} onClick={() => onSort(field, nextOrder)}>
        {label}
        <span className="sort-icon" aria-hidden>
          {active ? (order === "asc" ? "▲" : "▼") : "↕"}
        </span>
      </button>
    </th>
  );
}
