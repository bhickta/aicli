import type { WhatsAppContact, WorkflowDefinition } from "../types";

export const whatsappWorkflowDefinitions: WorkflowDefinition[] = [
  {
    id: "whatsapp-schedule",
    category: "WhatsApp",
    label: "Schedule message",
    endpoint: "/api/workflows/whatsapp/schedule",
    fields: [
      { type: "whatsappContact", id: "contact", label: "Contact" },
      { type: "textarea", id: "message", label: "Message", rows: 8, placeholder: "Message text" },
      { type: "text", id: "scheduled_at", label: "Send at", placeholder: "2026-05-20T18:30:00+05:30" },
      { type: "checkbox", id: "auto_send", label: "Auto-send from WhatsApp Web", checked: false },
      { type: "number", id: "wait_seconds", label: "WhatsApp load wait seconds", min: 3, max: 120, default: 12 },
    ],
    buildPayload: (values) => {
      const contact = (values.contact || {}) as Partial<WhatsAppContact>;
      return {
        recipient_name: contact.name || "",
        recipient_phone: contact.phone || "",
        recipient: contact.phone || contact.name || "",
        message: values.message,
        scheduled_at: values.scheduled_at,
        auto_send: Boolean(values.auto_send),
        wait_seconds: values.wait_seconds || 12,
      };
    },
  },
];
