import { state, WORKFLOW_PREFIX } from "../core/state.js";
import { escapeHtml, readNumberValue } from "../core/utils.js";
import { renderProviderModelRow, getProviderModelValues } from "../providers/controls.js";
import { workflowBrowseId, workflowControlId, workflowDisplayId } from "./utils.js";

export function renderWorkflowField(field) {
  if (field.type === "providerModel") {
    return renderProviderModelRow(WORKFLOW_PREFIX);
  }
  if (field.type === "text") {
    return renderTextField(field);
  }
  if (field.type === "textarea") {
    return renderTextareaField(field);
  }
  if (field.type === "select") {
    return renderSelectField(field);
  }
  if (field.type === "number") {
    return renderNumberField(field);
  }
  if (field.type === "checkbox") {
    return renderCheckboxField(field);
  }
  if (field.type === "path") {
    return renderPathField(field);
  }
  return "";
}

function renderTextField(field) {
  const id = workflowControlId(field.id);
  const value = field.value || "";
  return `
    <div class="field">
      <label for="${id}">${field.label}</label>
      <input id="${id}" type="text" value="${escapeHtml(value)}" placeholder="${escapeHtml(field.placeholder || "")}">
    </div>
  `;
}

function renderTextareaField(field) {
  const id = workflowControlId(field.id);
  return `
    <div class="field">
      <label for="${id}">${field.label}</label>
      <textarea id="${id}" rows="${field.rows || 6}" placeholder="${escapeHtml(field.placeholder || "")}"></textarea>
    </div>
  `;
}

function renderSelectField(field) {
  const id = workflowControlId(field.id);
  const options = field.options || [];
  return `
    <div class="field">
      <label for="${id}">${field.label}</label>
      <select id="${id}">
        ${options.map(renderSelectOption(field)).join("")}
      </select>
    </div>
  `;
}

function renderSelectOption(field) {
  return (option) => {
    const selected = option.value === (field.default ?? "") ? " selected" : "";
    return `<option value="${escapeHtml(option.value)}"${selected}>${escapeHtml(option.label)}</option>`;
  };
}

function renderNumberField(field) {
  const id = workflowControlId(field.id);
  const min = Number.isFinite(field.min) ? field.min : "";
  const max = Number.isFinite(field.max) ? field.max : "";
  const value = Number.isFinite(field.default) ? field.default : "";
  return `
    <div class="field">
      <label for="${id}">${field.label}</label>
      <input id="${id}" type="number" min="${min}" max="${max}" value="${escapeHtml(value)}">
    </div>
  `;
}

function renderCheckboxField(field) {
  const id = workflowControlId(field.id);
  const checked = field.checked ? " checked" : "";
  return `
    <div class="field">
      <label class="checkbox">
        <input id="${id}" type="checkbox" ${checked}>
        <span>${field.label}</span>
      </label>
    </div>
  `;
}

function renderPathField(field) {
  const browseId = workflowBrowseId(field.id);
  const valueId = workflowDisplayId(field.id);
  const currentPath = state.workflow.pathValues[field.id] || "";
  const chooseLabel = field.picker === "directory" ? `Browse ${field.label}` : `Choose ${field.label}`;
  return `
    <div class="field">
      <label for="${valueId}">${field.label}</label>
      <div class="path-control">
        <output id="${valueId}" data-path="${escapeHtml(currentPath)}">${escapeHtml(currentPath || `No ${field.label.toLowerCase()} selected`)}</output>
        <button type="button" id="${browseId}" data-workflow-target="${escapeHtml(field.id)}">${escapeHtml(chooseLabel)}</button>
      </div>
    </div>
  `;
}

export function collectWorkflowInputValues(definition) {
  const values = {};
  definition.fields.forEach((field) => {
    if (field.type === "providerModel") {
      Object.assign(values, getProviderModelValues(WORKFLOW_PREFIX));
      return;
    }
    if (field.type === "path") {
      const display = document.querySelector(`#${workflowDisplayId(field.id)}`);
      values[field.id] = display?.dataset.path || "";
      return;
    }
    const id = workflowControlId(field.id);
    const element = document.querySelector(`#${id}`);
    if (!element) return;
    if (field.type === "checkbox") {
      values[field.id] = element.checked;
      return;
    }
    if (field.type === "number") {
      values[field.id] = readNumberValue(element.value, field.default || 0, field.min || 0);
      return;
    }
    values[field.id] = element.value || "";
  });
  return values;
}

export function setWorkflowPathValue(target, pathValue) {
  state.workflow.pathValues[target] = pathValue || "";
  const display = document.querySelector(`#${workflowDisplayId(target)}`);
  if (!display) return;
  display.dataset.path = pathValue;
  display.textContent = pathValue || "No path selected";
}

export function buildWorkflowPayload(definition) {
  const inputValues = collectWorkflowInputValues(definition);
  if (definition.buildPayload) {
    return definition.buildPayload(inputValues);
  }
  return inputValues;
}
