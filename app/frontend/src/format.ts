const currencyFormatters = new Map<string, Intl.NumberFormat>();

export function formatMoney(value: number, currency = "BRL"): string {
  let formatter = currencyFormatters.get(currency);
  if (!formatter) {
    formatter = new Intl.NumberFormat(undefined, { style: "currency", currency });
    currencyFormatters.set(currency, formatter);
  }
  return formatter.format(value);
}

const dateTimeFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "short", timeStyle: "medium" });

export function formatDateTime(value: string | null): string {
  return value ? dateTimeFormatter.format(new Date(value)) : "—";
}

/** "credit_card" -> "Credit card" */
export function humanize(value: string): string {
  const text = value.replaceAll("_", " ");
  return text.charAt(0).toUpperCase() + text.slice(1);
}
