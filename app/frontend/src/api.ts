// Client for the backend API. Every list call sends the filters to the backend, which does the filtering.

export const API_BASE = import.meta.env.VITE_API_BASE ?? "/api";

export type SortOrder = "asc" | "desc";

export interface Page<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface Customer {
  customer_id: string;
  name: string;
  email: string;
  city: string;
  state: string;
}

export interface PurchaseItem {
  sku: string;
  name: string;
  category: string;
  unit_price: number;
  quantity: number;
  subtotal: number;
}

export interface Purchase {
  event_id: string;
  order_id: string;
  event_timestamp: string;
  status: string;
  channel: string;
  payment_method: string;
  installments: number;
  currency: string;
  subtotal: number;
  discount: number;
  shipping: number;
  total: number;
  source: string;
  received_at: string;
  customer: Customer;
  items: PurchaseItem[];
}

export interface CustomerSummary extends Customer {
  purchase_count: number;
  total_spent: number;
  last_purchase_at: string | null;
}

export interface FilterOptions {
  statuses: string[];
  channels: string[];
  payment_methods: string[];
  sources: string[];
  states: string[];
}

export interface PurchaseFilters {
  q?: string;
  status?: string;
  channel?: string;
  payment_method?: string;
  source?: string;
  customer_id?: string;
  /** Local date (YYYY-MM-DD), inclusive */
  from?: string;
  /** Local date (YYYY-MM-DD), inclusive */
  to?: string;
  min_total?: string;
  max_total?: string;
  sort?: string;
  order?: SortOrder;
  limit: number;
  offset: number;
}

export interface CustomerFilters {
  q?: string;
  state?: string;
  city?: string;
  sort?: string;
  order?: SortOrder;
  limit: number;
  offset: number;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

type QueryValue = string | number | undefined | null;

/** Builds a query string, leaving out empty values. */
export function toQueryString(params: Record<string, QueryValue>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    search.set(key, String(value));
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

/** Start of a local calendar day as an ISO timestamp, optionally shifted by whole days. */
export function localDayStart(date: string, addDays = 0): string {
  const [year, month, day] = date.split("-").map(Number);
  return new Date(year, month - 1, day + addDays).toISOString();
}

/** Maps the purchase filters to backend query parameters; the date range covers whole local days. */
export function purchaseParams(filters: PurchaseFilters): Record<string, QueryValue> {
  const { from, to, ...rest } = filters;
  return {
    ...rest,
    from: from ? localDayStart(from) : undefined,
    // The backend "to" is exclusive, so the end of the range is the start of the next day
    to: to ? localDayStart(to, 1) : undefined,
  };
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, { signal, headers: { Accept: "application/json" } });
  if (!response.ok) {
    let detail = response.statusText;
    try {
      detail = ((await response.json()) as { detail?: string }).detail ?? detail;
    } catch {
      // Not a JSON error body
    }
    throw new ApiError(detail || `Request failed with status ${response.status}`, response.status);
  }
  return (await response.json()) as T;
}

export function listPurchases(filters: PurchaseFilters, signal?: AbortSignal): Promise<Page<Purchase>> {
  return getJSON(`/purchases${toQueryString(purchaseParams(filters))}`, signal);
}

export function listCustomers(filters: CustomerFilters, signal?: AbortSignal): Promise<Page<CustomerSummary>> {
  return getJSON(`/customers${toQueryString({ ...filters })}`, signal);
}

export function getFilterOptions(signal?: AbortSignal): Promise<FilterOptions> {
  return getJSON("/filters", signal);
}
