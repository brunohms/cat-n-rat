import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Fragment, useState } from "react";
import { getFilterOptions, listPurchases, type PurchaseFilters, type SortOrder } from "../api";
import { DebouncedInput } from "../components/DebouncedInput";
import { Field, SelectField } from "../components/Field";
import { Pagination } from "../components/Pagination";
import { QueryState } from "../components/QueryState";
import { SortHeader } from "../components/SortHeader";
import { formatDateTime, formatMoney, humanize } from "../format";
import type { ParamChanges } from "../useUrlParams";

const FILTER_KEYS = [
  "q",
  "status",
  "channel",
  "payment_method",
  "source",
  "customer_id",
  "from",
  "to",
  "min_total",
  "max_total",
] as const;

export function readPurchaseFilters(params: URLSearchParams): PurchaseFilters {
  const filters: PurchaseFilters = {
    sort: params.get("sort") ?? "event_timestamp",
    order: params.get("order") === "asc" ? "asc" : "desc",
    limit: Number(params.get("limit")) || 50,
    offset: Number(params.get("offset")) || 0,
  };
  for (const key of FILTER_KEYS) {
    filters[key] = params.get(key) ?? "";
  }
  return filters;
}

interface Props {
  params: URLSearchParams;
  update: (changes: ParamChanges) => void;
  autoRefresh: boolean;
}

export function PurchasesView({ params, update, autoRefresh }: Props) {
  const filters = readPurchaseFilters(params);
  const [expanded, setExpanded] = useState<string | null>(null);

  const purchases = useQuery({
    queryKey: ["purchases", filters],
    queryFn: ({ signal }) => listPurchases(filters, signal),
    placeholderData: keepPreviousData,
    refetchInterval: autoRefresh ? 5000 : false,
  });
  const options = useQuery({
    queryKey: ["filter-options"],
    queryFn: ({ signal }) => getFilterOptions(signal),
    staleTime: 60_000,
  });

  // Any filter change goes back to the first page
  const setFilter = (key: string) => (value: string) => update({ [key]: value, offset: null });
  const onSort = (sort: string, order: SortOrder) => update({ sort, order, offset: null });
  const hasFilters = FILTER_KEYS.some((key) => filters[key]);
  const sortProps = { sort: filters.sort!, order: filters.order!, onSort };

  return (
    <section>
      <form className="filters" onSubmit={(event) => event.preventDefault()} aria-label="Purchase filters">
        <Field label="Search">
          <DebouncedInput
            type="search"
            placeholder="Order, customer name or email"
            value={filters.q!}
            onChange={setFilter("q")}
          />
        </Field>
        <SelectField
          label="Status"
          value={filters.status!}
          options={options.data?.statuses ?? []}
          onChange={setFilter("status")}
          format={humanize}
        />
        <SelectField
          label="Channel"
          value={filters.channel!}
          options={options.data?.channels ?? []}
          onChange={setFilter("channel")}
          format={humanize}
        />
        <SelectField
          label="Payment"
          value={filters.payment_method!}
          options={options.data?.payment_methods ?? []}
          onChange={setFilter("payment_method")}
          format={humanize}
        />
        <SelectField
          label="Source"
          value={filters.source!}
          options={options.data?.sources ?? []}
          onChange={setFilter("source")}
          format={humanize}
        />
        <Field label="From">
          <input type="date" value={filters.from} max={filters.to || undefined} onChange={(e) => setFilter("from")(e.target.value)} />
        </Field>
        <Field label="To">
          <input type="date" value={filters.to} min={filters.from || undefined} onChange={(e) => setFilter("to")(e.target.value)} />
        </Field>
        <Field label="Min total">
          <DebouncedInput type="number" min={0} step="0.01" inputMode="decimal" value={filters.min_total!} onChange={setFilter("min_total")} />
        </Field>
        <Field label="Max total">
          <DebouncedInput type="number" min={0} step="0.01" inputMode="decimal" value={filters.max_total!} onChange={setFilter("max_total")} />
        </Field>
        <Field label="Customer ID">
          <DebouncedInput placeholder="CUS-000000" value={filters.customer_id!} onChange={setFilter("customer_id")} />
        </Field>
        <button
          type="button"
          className="secondary"
          disabled={!hasFilters}
          onClick={() => update(Object.fromEntries([...FILTER_KEYS, "offset"].map((key) => [key, null])))}
        >
          Clear filters
        </button>
      </form>

      <div className={`table-wrap ${purchases.isFetching ? "fetching" : ""}`}>
        <table>
          <thead>
            <tr>
              <SortHeader label="Date" field="event_timestamp" defaultOrder="desc" {...sortProps} />
              <SortHeader label="Order" field="order_id" {...sortProps} />
              <SortHeader label="Customer" field="customer" {...sortProps} />
              <SortHeader label="Status" field="status" {...sortProps} />
              <th>Channel</th>
              <th>Payment</th>
              <th className="num">Items</th>
              <SortHeader label="Total" field="total" defaultOrder="desc" align="right" {...sortProps} />
              <th>Source</th>
            </tr>
          </thead>
          <QueryState
            isLoading={purchases.isPending}
            error={purchases.error}
            isEmpty={purchases.data?.data.length === 0}
            colSpan={9}
          >
            <tbody>
              {purchases.data?.data.map((p) => {
                const isOpen = expanded === p.event_id;
                return (
                  <Fragment key={p.event_id}>
                    <tr
                      className={`row ${isOpen ? "open" : ""}`}
                      onClick={() => setExpanded(isOpen ? null : p.event_id)}
                      aria-expanded={isOpen}
                    >
                      <td className="nowrap">{formatDateTime(p.event_timestamp)}</td>
                      <td className="mono">{p.order_id}</td>
                      <td>
                        <div>{p.customer.name}</div>
                        <div className="muted small">{p.customer.email}</div>
                      </td>
                      <td>
                        <span className={`badge status-${p.status}`}>{humanize(p.status)}</span>
                      </td>
                      <td>{humanize(p.channel)}</td>
                      <td>
                        {humanize(p.payment_method)}
                        {p.installments > 1 && <span className="muted"> · {p.installments}x</span>}
                      </td>
                      <td className="num">{p.items.reduce((sum, item) => sum + item.quantity, 0)}</td>
                      <td className="num nowrap">{formatMoney(p.total, p.currency)}</td>
                      <td>
                        <span className="badge">{p.source}</span>
                      </td>
                    </tr>
                    {isOpen && (
                      <tr className="details">
                        <td colSpan={9}>
                          <table className="items">
                            <thead>
                              <tr>
                                <th>SKU</th>
                                <th>Product</th>
                                <th>Category</th>
                                <th className="num">Unit price</th>
                                <th className="num">Qty</th>
                                <th className="num">Subtotal</th>
                              </tr>
                            </thead>
                            <tbody>
                              {p.items.map((item, index) => (
                                <tr key={index}>
                                  <td className="mono">{item.sku}</td>
                                  <td>{item.name}</td>
                                  <td>{humanize(item.category)}</td>
                                  <td className="num">{formatMoney(item.unit_price, p.currency)}</td>
                                  <td className="num">{item.quantity}</td>
                                  <td className="num">{formatMoney(item.subtotal, p.currency)}</td>
                                </tr>
                              ))}
                            </tbody>
                            <tfoot>
                              <tr>
                                <td colSpan={5}>Subtotal · discount · shipping</td>
                                <td className="num nowrap">
                                  {formatMoney(p.subtotal, p.currency)} · −{formatMoney(p.discount, p.currency)} · +
                                  {formatMoney(p.shipping, p.currency)}
                                </td>
                              </tr>
                            </tfoot>
                          </table>
                          <p className="muted small">
                            {p.customer.city}/{p.customer.state} · customer {p.customer.customer_id} · event {p.event_id} ·
                            received {formatDateTime(p.received_at)}
                          </p>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </QueryState>
        </table>
      </div>

      {purchases.data && (
        <Pagination
          total={purchases.data.total}
          limit={filters.limit}
          offset={filters.offset}
          onChange={(offset, limit) => update({ offset: offset || null, limit: limit === 50 ? null : limit })}
        />
      )}
    </section>
  );
}
