import { useEffect, useState, type InputHTMLAttributes } from "react";

interface Props extends Omit<InputHTMLAttributes<HTMLInputElement>, "value" | "onChange"> {
  value: string;
  onChange: (value: string) => void;
  delay?: number;
}

/** Text input that reports changes only after the user stops typing, to avoid one request per keystroke. */
export function DebouncedInput({ value, onChange, delay = 350, ...props }: Props) {
  const [text, setText] = useState(value);

  // Follows external changes, e.g. "Clear filters" or the back button
  useEffect(() => setText(value), [value]);

  useEffect(() => {
    if (text === value) return;
    const timer = setTimeout(() => onChange(text), delay);
    return () => clearTimeout(timer);
  }, [text, value, delay, onChange]);

  return <input {...props} value={text} onChange={(event) => setText(event.target.value)} />;
}
