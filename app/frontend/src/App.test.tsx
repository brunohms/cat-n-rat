import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { CustomerSummary, FilterOptions, Page, Purchase } from "./api";
import { App } from "./App";

const purchase: Purchase = {
  event_id: "11111111-1111-4111-8111-111111111111",
  order_id: "ORD-00000001",
  event_timestamp: "2026-10-22T12:00:00Z",
  status: "paid",
  channel: "web",
  payment_method: "credit_card",
  installments: 3,
  currency: "BRL",
  subtotal: 100,
  discount: 10,
  shipping: 5,
  total: 95,
  source: "push",
  received_at: "2026-10-22T12:00:01Z",
  customer: { customer_id: "CUS-000001", name: "Ana Souza", email: "ana@example.com", city: "Recife", state: "PE" },
  items: [{ sku: "CAT-TOY-01", name: "Plush mouse", category: "toys", unit_price: 50, quantity: 2, subtotal: 100 }],
};

const customer: CustomerSummary = {
  ...purchase.customer,
  purchase_count: 1,
  total_spent: 95,
  last_purchase_at: purchase.event_timestamp,
};

const options: FilterOptions = {
  statuses: ["cancelled", "paid"],
  channels: ["store", "web"],
  payment_methods: ["credit_card", "pix"],
  sources: ["pull", "push"],
  states: ["PE", "SP"],
};

function page<T>(data: T[], url: URL): Page<T> {
  return { data, total: data.length, limit: Number(url.searchParams.get("limit")), offset: Number(url.searchParams.get("offset")) };
}

let fetchMock: ReturnType<typeof vi.fn>;

/** Query parameters of the requests sent to a backend path. */
function requestsTo(path: string): URLSearchParams[] {
  return fetchMock.mock.calls
    .map(([input]) => new URL(String(input), "http://localhost"))
    .filter((url) => url.pathname === `/api${path}`)
    .map((url) => url.searchParams);
}

function lastRequestTo(path: string): URLSearchParams {
  const requests = requestsTo(path);
  expect(requests.length).toBeGreaterThan(0);
  return requests[requests.length - 1];
}

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  window.history.replaceState(null, "", "/");
  fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input), "http://localhost");
    const body =
      url.pathname === "/api/purchases"
        ? page([purchase], url)
        : url.pathname === "/api/customers"
          ? page([customer], url)
          : options;
    return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("purchases view", () => {
  it("loads the first page sorted by date", async () => {
    renderApp();

    expect(await screen.findByText("ORD-00000001")).toBeInTheDocument();
    const params = lastRequestTo("/purchases");
    expect(Object.fromEntries(params)).toEqual({ sort: "event_timestamp", order: "desc", limit: "50", offset: "0" });
  });

  it("sends every select filter to the backend", async () => {
    const user = userEvent.setup();
    renderApp();
    await screen.findByText("ORD-00000001");
    await screen.findByRole("option", { name: "Paid" });

    await user.selectOptions(screen.getByLabelText("Status"), "paid");
    await waitFor(() => expect(lastRequestTo("/purchases").get("status")).toBe("paid"));

    await user.selectOptions(screen.getByLabelText("Payment"), "pix");
    await waitFor(() => expect(lastRequestTo("/purchases").get("payment_method")).toBe("pix"));
    expect(lastRequestTo("/purchases").get("status")).toBe("paid");
    expect(window.location.search).toContain("status=paid");
  });

  it("sends the search text after the user stops typing", async () => {
    const user = userEvent.setup();
    renderApp();
    await screen.findByText("ORD-00000001");

    await user.type(screen.getByLabelText("Search"), "souza");

    await waitFor(() => expect(lastRequestTo("/purchases").get("q")).toBe("souza"));
    // Debounced: no request for the partial text
    expect(requestsTo("/purchases").some((params) => params.get("q") === "s")).toBe(false);
  });

  it("sends the date range as local day boundaries", async () => {
    const user = userEvent.setup();
    renderApp();
    await screen.findByText("ORD-00000001");

    await user.type(screen.getByLabelText("From"), "2026-10-01");

    await waitFor(() => expect(lastRequestTo("/purchases").get("from")).toBe(new Date(2026, 9, 1).toISOString()));
  });

  it("sorts on the backend when a column header is clicked", async () => {
    const user = userEvent.setup();
    renderApp();
    await screen.findByText("ORD-00000001");

    await user.click(screen.getByRole("button", { name: /Total/ }));
    await waitFor(() => expect(lastRequestTo("/purchases").get("sort")).toBe("total"));
    expect(lastRequestTo("/purchases").get("order")).toBe("desc");

    await user.click(screen.getByRole("button", { name: /Total/ }));
    await waitFor(() => expect(lastRequestTo("/purchases").get("order")).toBe("asc"));
  });

  it("resets the page when a filter changes", async () => {
    window.history.replaceState(null, "", "/?offset=50");
    const user = userEvent.setup();
    renderApp();
    await screen.findByText("ORD-00000001");
    expect(lastRequestTo("/purchases").get("offset")).toBe("50");

    await user.selectOptions(screen.getByLabelText("Channel"), "web");

    await waitFor(() => expect(lastRequestTo("/purchases").get("channel")).toBe("web"));
    expect(lastRequestTo("/purchases").get("offset")).toBe("0");
  });

  it("shows the items when a row is clicked", async () => {
    const user = userEvent.setup();
    renderApp();

    await user.click(await screen.findByText("ORD-00000001"));

    expect(screen.getByText("Plush mouse")).toBeInTheDocument();
  });

  it("shows the backend error", async () => {
    fetchMock.mockImplementation(async () => new Response(JSON.stringify({ detail: "database unavailable" }), { status: 503 }));
    renderApp();

    expect(await screen.findByText(/database unavailable/)).toBeInTheDocument();
  });
});

describe("customers view", () => {
  it("filters customers on the backend and links to their purchases", async () => {
    const user = userEvent.setup();
    renderApp();
    await screen.findByText("ORD-00000001");

    await user.click(screen.getByRole("tab", { name: "Customers" }));
    const row = (await screen.findByText("CUS-000001")).closest("tr")!;
    expect(Object.fromEntries(lastRequestTo("/customers"))).toEqual({ sort: "name", order: "asc", limit: "50", offset: "0" });

    await user.selectOptions(await screen.findByLabelText("State"), "PE");
    await waitFor(() => expect(lastRequestTo("/customers").get("state")).toBe("PE"));

    await user.click(within(row).getByRole("button", { name: "View purchases" }));
    await waitFor(() => expect(lastRequestTo("/purchases").get("customer_id")).toBe("CUS-000001"));
    expect(screen.getByRole("tab", { name: "Purchases" })).toHaveAttribute("aria-selected", "true");
  });
});
