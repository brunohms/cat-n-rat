import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { getFilterOptions, listCustomers, type CustomerFilters, type SortOrder } from "../api";
import { DebouncedInput } from "../components/DebouncedInput";
import { Field, SelectField } from "../components/Field";
import { Pagination } from "../components/Pagination";
import { QueryState } from "../components/QueryState";
import { SortHeader } from "../components/SortHeader";
import { formatDateTime, formatMoney } from "../format";
import type { ParamChanges } from "../useUrlParams";

const FILTER_KEYS = ["q", "state", "city"] as const;

export function readCustomerFilters(params: URLSearchParams): CustomerFilters {
  return {
    q: params.get("q") ?? "",
    state: params.get("state") ?? "",
    city: params.get("city") ?? "",
    sort: params.get("sort") ?? "name",
    order: params.get("order") === "desc" ? "desc" : "asc",
    limit: Number(params.get("limit")) || 50,
    offset: Number(params.get("offset")) || 0,
  };
}

interface Props {
  params: URLSearchParams;
  update: (changes: ParamChanges) => void;
  autoRefresh: boolean;
  onShowPurchases: (customerId: string) => void;
}

export function CustomersView({ params, update, autoRefresh, onShowPurchases }: Props) {
  const filters = readCustomerFilters(params);

  const customers = useQuery({
    queryKey: ["customers", filters],
    queryFn: ({ signal }) => listCustomers(filters, signal),
    placeholderData: keepPreviousData,
    refetchInterval: autoRefresh ? 5000 : false,
  });
  const options = useQuery({
    queryKey: ["filter-options"],
    queryFn: ({ signal }) => getFilterOptions(signal),
    staleTime: 60_000,
  });

  const setFilter = (key: string) => (value: string) => update({ [key]: value, offset: null });
  const onSort = (sort: string, order: SortOrder) => update({ sort, order, offset: null });
  const hasFilters = FILTER_KEYS.some((key) => filters[key]);
  const sortProps = { sort: filters.sort!, order: filters.order!, onSort };

  return (
    <section>
      <form className="filters" onSubmit={(event) => event.preventDefault()} aria-label="Customer filters">
        <Field label="Search">
          <DebouncedInput type="search" placeholder="ID, name or email" value={filters.q!} onChange={setFilter("q")} />
        </Field>
        <SelectField label="State" value={filters.state!} options={options.data?.states ?? []} onChange={setFilter("state")} />
        <Field label="City">
          <DebouncedInput placeholder="City" value={filters.city!} onChange={setFilter("city")} />
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

      <div className={`table-wrap ${customers.isFetching ? "fetching" : ""}`}>
        <table>
          <thead>
            <tr>
              <SortHeader label="Customer" field="name" {...sortProps} />
              <SortHeader label="ID" field="customer_id" {...sortProps} />
              <th>City</th>
              <SortHeader label="State" field="state" {...sortProps} />
              <SortHeader label="Purchases" field="purchase_count" defaultOrder="desc" align="right" {...sortProps} />
              <SortHeader label="Total spent (paid)" field="total_spent" defaultOrder="desc" align="right" {...sortProps} />
              <SortHeader label="Last purchase" field="last_purchase_at" defaultOrder="desc" {...sortProps} />
              <th />
            </tr>
          </thead>
          <QueryState
            isLoading={customers.isPending}
            error={customers.error}
            isEmpty={customers.data?.data.length === 0}
            colSpan={8}
          >
            <tbody>
              {customers.data?.data.map((c) => (
                <tr key={c.customer_id}>
                  <td>
                    <div>{c.name}</div>
                    <div className="muted small">{c.email}</div>
                  </td>
                  <td className="mono">{c.customer_id}</td>
                  <td>{c.city}</td>
                  <td>{c.state}</td>
                  <td className="num">{c.purchase_count}</td>
                  <td className="num nowrap">{formatMoney(c.total_spent)}</td>
                  <td className="nowrap">{formatDateTime(c.last_purchase_at)}</td>
                  <td>
                    <button type="button" className="link" onClick={() => onShowPurchases(c.customer_id)}>
                      View purchases
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </QueryState>
        </table>
      </div>

      {customers.data && (
        <Pagination
          total={customers.data.total}
          limit={filters.limit}
          offset={filters.offset}
          onChange={(offset, limit) => update({ offset: offset || null, limit: limit === 50 ? null : limit })}
        />
      )}
    </section>
  );
}
