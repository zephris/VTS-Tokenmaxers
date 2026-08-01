<script setup lang="ts">
import { DiceOutline, KeyOutline, RadioOutline, ServerOutline, ShieldCheckmarkOutline, TrashOutline } from '@vicons/ionicons5';
import { computed, ref, watch } from 'vue';
import { NAlert, NButton, NCard, NFormItem, NIcon, NInput, NTag } from 'naive-ui';
import type { AuthAccount, HealthResponse } from '@vts/common';

const props = defineProps<{
  savedPhrase: string;
  health?: HealthResponse;
  account?: AuthAccount;
}>();

const emit = defineEmits<{ savePhrase: [value: string]; clearPhrase: [] }>();
const draft = ref(props.savedPhrase);
const savedNotice = ref(false);
const phraseValid = computed(() => draft.value.length >= 16);
const phraseWords = [
  'anchor', 'apricot', 'atlas', 'beacon', 'birch', 'breeze', 'canyon', 'cedar',
  'cinder', 'cobalt', 'comet', 'coral', 'cricket', 'dawn', 'delta', 'ember',
  'fern', 'fjord', 'garden', 'glacier', 'harbor', 'hazel', 'horizon', 'indigo',
  'iris', 'island', 'juniper', 'lantern', 'lilac', 'lumen', 'maple', 'meadow',
  'meridian', 'meteor', 'moss', 'nebula', 'orchard', 'pebble', 'quartz', 'raven',
  'reef', 'relay', 'river', 'saffron', 'signal', 'solar', 'sparrow', 'spruce',
  'summit', 'thistle', 'timber', 'valley', 'velvet', 'violet', 'willow', 'zephyr',
] as const;
watch(() => props.savedPhrase, (value) => { draft.value = value; });

function generatePhrase() {
  const words = new Set<string>();
  while (words.size < 6) {
    const random = new Uint32Array(1);
    window.crypto.getRandomValues(random);
    words.add(phraseWords[(random[0] ?? 0) % phraseWords.length] ?? phraseWords[0]);
  }
  draft.value = [...words].join(' ');
  savedNotice.value = false;
}

function save() {
  if (!phraseValid.value) return;
  emit('savePhrase', draft.value);
  savedNotice.value = true;
  window.setTimeout(() => { savedNotice.value = false; }, 2500);
}

function clear() {
  draft.value = '';
  savedNotice.value = false;
  emit('clearPhrase');
}
</script>

<template>
  <div class="view-stack settings-view">
    <header class="view-header">
      <div>
        <span class="eyebrow">Operator preferences</span>
        <h1>Settings</h1>
        <p>Configure the phrase used by the secure channel in this browser tab.</p>
      </div>
    </header>

    <NCard title="Station identity">
      <template #header-extra><NIcon :component="RadioOutline" size="20" /></template>
      <dl class="status-list">
        <div><dt>Signed-in station</dt><dd>{{ account?.stationId ?? 'Unknown' }}</dd></div>
        <div><dt>Username</dt><dd>{{ account?.username ?? 'Unknown' }}</dd></div>
        <div><dt>Session</dt><dd><NTag type="success" :bordered="false">Active</NTag></dd></div>
      </dl>
    </NCard>

    <NCard title="Secret phrase">
      <template #header-extra><NIcon :component="KeyOutline" size="20" /></template>
      <NFormItem
        label="Encryption phrase"
        :validation-status="draft.length > 0 && !phraseValid ? 'error' : undefined"
        :feedback="draft.length > 0 && !phraseValid ? `${16 - draft.length} more characters required` : undefined"
      >
        <NInput
          v-model:value="draft"
          type="password"
          show-password-on="click"
          placeholder="At least 16 characters"
          autocomplete="off"
          maxlength="160"
          @keyup.enter="save"
        />
      </NFormItem>
      <div class="settings-actions">
        <NButton secondary @click="generatePhrase">
          <template #icon><NIcon :component="DiceOutline" /></template>
          Generate phrase
        </NButton>
        <NButton type="primary" :disabled="!phraseValid" @click="save">
          <template #icon><NIcon :component="ShieldCheckmarkOutline" /></template>
          Save for this tab
        </NButton>
        <NButton secondary @click="clear">
          <template #icon><NIcon :component="TrashOutline" /></template>
          Clear
        </NButton>
      </div>
      <NAlert v-if="savedNotice" type="success" class="settings-alert">Phrase saved for this browser tab.</NAlert>
      <NAlert type="warning" class="settings-alert">
        The phrase is stored in session storage and cleared when this tab is closed. It is sent only to the secure-channel API for the current encode or decode request and is never persisted by the server.
      </NAlert>
    </NCard>

    <NCard title="Runtime status">
      <template #header-extra><NIcon :component="ServerOutline" size="20" /></template>
      <dl class="status-list">
        <div><dt>Server mode</dt><dd><NTag :type="health?.mode === 'full' ? 'success' : 'warning'" :bordered="false">{{ health?.mode ?? 'unknown' }}</NTag></dd></div>
        <div><dt>Dataset</dt><dd>{{ health ? `${health.data.broadcastCount} broadcasts across ${health.data.stationCount} stations` : 'Unavailable' }}</dd></div>
        <div><dt>Steganography</dt><dd>{{ health?.steganography.configured ? 'Model configured' : 'Model disabled' }}</dd></div>
        <div><dt>Algorithm</dt><dd>{{ health?.steganography.algorithm || 'Not available' }}</dd></div>
      </dl>
    </NCard>
  </div>
</template>

<style scoped>
.settings-view { max-width: 760px; }
.settings-actions { display: flex; flex-wrap: wrap; gap: 10px; }
.settings-alert { margin-top: 14px; }
.status-list { margin: 0; }
.status-list > div { display: grid; padding: 12px 0; border-bottom: 1px solid var(--border); grid-template-columns: 160px minmax(0, 1fr); gap: 20px; }
.status-list > div:last-child { border-bottom: 0; }
.status-list dt { color: var(--text-faint); }
.status-list dd { margin: 0; color: var(--text); text-align: right; overflow-wrap: anywhere; }
@media (max-width: 540px) { .status-list > div { grid-template-columns: 1fr; gap: 5px; } .status-list dd { text-align: left; } }
</style>
