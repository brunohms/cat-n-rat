import { useState } from "react";
import { useUrlParams } from "./useUrlParams";
import { CustomersView } from "./views/CustomersView";
import { PurchasesView } from "./views/PurchasesView";

const VIEWS = [
  { id: "purchases", label: "Purchases" },
  { id: "customers", label: "Customers" },
] as const;

export function App() {
  const [params, update] = useUrlParams();
  const [autoRefresh, setAutoRefresh] = useState(false);
  const view = params.get("view") === "customers" ? "customers" : "purchases";

  return (
    <div className="app">
      <header className="header">
        <h1>Cat n Rat</h1>
        <nav className="tabs" role="tablist">
          {VIEWS.map(({ id, label }) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={view === id}
              className={view === id ? "active" : undefined}
              // Each view has its own filters, so switching views starts from a clean query string
              onClick={() => update({ view: id === "purchases" ? null : id }, { reset: true })}
            >
              {label}
            </button>
          ))}
        </nav>
        <label className="toggle">
          <input type="checkbox" checked={autoRefresh} onChange={(event) => setAutoRefresh(event.target.checked)} />
          Auto-refresh (5s)
        </label>
      </header>

      <main>
        {view === "purchases" ? (
          <PurchasesView params={params} update={update} autoRefresh={autoRefresh} />
        ) : (
          <CustomersView
            params={params}
            update={update}
            autoRefresh={autoRefresh}
            onShowPurchases={(customerId) => update({ customer_id: customerId }, { reset: true })}
          />
        )}
      </main>
    </div>
  );
}
