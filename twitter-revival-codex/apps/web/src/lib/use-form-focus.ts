"use client";
import { useEffect, useRef } from "react";

// Native validation handles browser constraints; this covers API and custom errors.
export function useFormFocus(
  pending: boolean,
  fields: Record<string, string>,
  error: string,
) {
  const form = useRef<HTMLFormElement>(null);
  useEffect(() => {
    if (pending || (!error && Object.keys(fields).length === 0)) return;
    const target = form.current?.querySelector<HTMLElement>(
      '[aria-invalid="true"], [role="alert"][tabindex="-1"]',
    );
    target?.focus();
  }, [pending, fields, error]);
  return form;
}
