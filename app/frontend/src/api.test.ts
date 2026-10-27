import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, listPurchases, localDayStart, purchaseParams, toQueryString } from "./api";

describe("toQueryString", () => {
  it("leaves out empty values", () => {
    expect(toQueryString({ q: "", status: "paid", customer_id: undefined, min_total: null, limit: 50, offset: 0 })).toBe(
      "?status=paid&limit=50&offset=0",
    );
  });

  it("returns an empty string without parameters", () => {
    expect(toQueryString({ q: "" })).toBe("");
  });

  it("encodes special characters", () => {
    expect(toQueryString({ q: "a&b %" })).toBe("?q=a%26b+%25");
  });
});

describe("purchaseParams", () => {
  it("turns the inclusive local date range into an exclusive timestamp range", () => {
    const params = purchaseParams({ from: "2026-10-01", to: "2026-10-31", limit: 50, offset: 0 });

    expect(params.from).toBe(new Date(2026, 9, 1).toISOString());
    expect(params.to).toBe(new Date(2026, 10, 1).toISOString());
  });

  it("keeps the other filters unchanged", () => {
    expect(purchaseParams({ status: "paid", sort: "total", order: "asc", limit: 25, offset: 50 })).toMatchObject({
      status: "paid",
      sort: "total",
      order: "asc",
      limit: 25,
      offset: 50,
      from: undefined,
      to: undefined,
    });
  });
});

describe("localDayStart", () => {
  it("rolls over month boundaries", () => {
    expect(localDayStart("2026-12-31", 1)).toBe(new Date(2027, 0, 1).toISOString());
  });
});

describe("listPurchases", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("raises the backend error detail", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify({ detail: "sort must be one of: total" }), { status: 422 })),
    );

    const error = await listPurchases({ sort: "bad", limit: 50, offset: 0 }).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ message: "sort must be one of: total", status: 422 });
  });
});
