import type { WorkflowDefinition } from "../types";

type Values = Record<string, unknown>;

export const providerModelField = { type: "providerModel" } satisfies WorkflowDefinition["fields"][number];

export function providerModelPayload(values: Values) {
  return {
    provider_id: values.provider_id,
    model: values.model,
  };
}

export function pathPayload(values: Values, key = "path") {
  return { [key]: values[key] };
}

export function providerPathPayload(values: Values, pathKey = "path") {
  return {
    ...providerModelPayload(values),
    ...pathPayload(values, pathKey),
  };
}

export function documentPayload(values: Values) {
  return {
    ...providerPathPayload(values),
    render_workers: values.render_workers,
    workers: values.workers,
  };
}
