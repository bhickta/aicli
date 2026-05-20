<script setup lang="ts">
import { computed, shallowRef, watch } from "vue";
import { appState } from "../../stores/appState";
import type { WhatsAppContact, WorkflowField } from "../../types";

const props = defineProps<{
  field: WorkflowField;
  value: unknown;
}>();

const emit = defineEmits<{
  update: [id: string, value: WhatsAppContact];
}>();

const selectedID = shallowRef("");
const draftName = shallowRef("");
const draftPhone = shallowRef("");

const contacts = computed(() => appState.whatsapp.contacts);

watch(() => props.value, hydrateFromValue, { immediate: true });

function hydrateFromValue() {
  const value = props.value as Partial<WhatsAppContact> | undefined;
  selectedID.value = value?.id || selectedID.value;
  draftName.value = value?.name || draftName.value;
  draftPhone.value = value?.phone || draftPhone.value;
}

function selectContact(id: string) {
  selectedID.value = id;
  if (!id) {
    draftName.value = "";
    draftPhone.value = "";
    updateDraft();
    return;
  }
  const contact = contacts.value.find((item) => item.id === id);
  if (!contact) return;
  draftName.value = contact.name;
  draftPhone.value = contact.phone;
  emitContact(contact);
}

function updateDraft() {
  emitContact({
    id: selectedID.value,
    name: draftName.value.trim(),
    phone: draftPhone.value.trim(),
  });
}

function saveContact() {
  const name = draftName.value.trim();
  const phone = draftPhone.value.trim();
  if (!name || !phone) return;
  const id = selectedID.value || newContactID();
  const contact = { id, name, phone };
  const index = appState.whatsapp.contacts.findIndex((item) => item.id === id);
  if (index >= 0) {
    appState.whatsapp.contacts[index] = contact;
  } else {
    appState.whatsapp.contacts.push(contact);
  }
  selectedID.value = id;
  emitContact(contact);
}

function removeContact() {
  if (!selectedID.value) return;
  appState.whatsapp.contacts = appState.whatsapp.contacts.filter((item) => item.id !== selectedID.value);
  selectedID.value = "";
  draftName.value = "";
  draftPhone.value = "";
  updateDraft();
}

function emitContact(contact: WhatsAppContact) {
  if (!props.field.id) return;
  emit("update", props.field.id, contact);
}

function newContactID() {
  return globalThis.crypto?.randomUUID?.() || `wa-${Date.now()}`;
}
</script>

<template>
  <div v-if="field.id" class="field whatsapp-contact-field">
    <label :for="`wf-${field.id}-select`">{{ field.label }}</label>
    <div class="field-row">
      <div class="field">
        <label :for="`wf-${field.id}-select`">Select saved contact</label>
        <select :id="`wf-${field.id}-select`" :value="selectedID" @change="selectContact(($event.target as HTMLSelectElement).value)">
          <option value="">New contact</option>
          <option v-if="!contacts.length" value="" disabled>No saved contacts yet</option>
          <option v-for="contact in contacts" :key="contact.id" :value="contact.id">
            {{ contact.name }} · {{ contact.phone }}
          </option>
        </select>
      </div>
      <div class="field">
        <label :for="`wf-${field.id}-name`">Contact name</label>
        <input
          :id="`wf-${field.id}-name`"
          :value="draftName"
          type="text"
          placeholder="Name"
          @input="draftName = ($event.target as HTMLInputElement).value; updateDraft()"
        >
      </div>
      <div class="field">
        <label :for="`wf-${field.id}-phone`">Phone</label>
        <input
          :id="`wf-${field.id}-phone`"
          :value="draftPhone"
          type="tel"
          placeholder="+919999999999"
          @input="draftPhone = ($event.target as HTMLInputElement).value; updateDraft()"
        >
      </div>
    </div>
    <div class="field-row compact-actions">
      <button type="button" @click="saveContact">Save contact</button>
      <button type="button" :disabled="!selectedID" @click="removeContact">Remove contact</button>
    </div>
  </div>
</template>

<style scoped>
.whatsapp-contact-field {
  gap: 10px;
}

.compact-actions {
  justify-content: flex-start;
}
</style>
