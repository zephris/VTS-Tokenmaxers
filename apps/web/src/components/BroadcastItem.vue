<script setup lang="ts">
import {
  CellularOutline,
  CheckmarkCircleOutline,
  LocationOutline,
  LockClosedOutline,
  LockOpenOutline,
  RadioOutline,
  TimeOutline,
} from '@vicons/ionicons5';
import { computed } from 'vue';
import { NIcon, NTag } from 'naive-ui';
import type { Broadcast } from '@vts/common';
import type { DisplayMode } from '../composables/useSessionPreferences';
import { formatTimestamp, titleCase } from '../utils/format';

const props = defineProps<{
  broadcast: Broadcast;
  displayMode: DisplayMode;
}>();

const shownText = computed(() => {
  if (
    props.displayMode === 'carrier' &&
    props.broadcast.encryptionStatus === 'encrypted' &&
    props.broadcast.carrierText
  ) {
    return props.broadcast.carrierText;
  }
  return props.broadcast.messageText;
});

const signalType = computed(() => {
  const signal = props.broadcast.signalStrength ?? 0;
  if (signal >= 65) return 'success';
  if (signal >= 35) return 'warning';
  return 'error';
});

const checkType = computed(() => {
  if (props.broadcast.crossCheckStatus === 'verified') return 'success';
  if (props.broadcast.crossCheckStatus === 'disputed') return 'error';
  return 'warning';
});
</script>

<template>
  <article :class="['broadcast-item', { encrypted: broadcast.encryptionStatus === 'encrypted' }]">
    <div class="broadcast-rail">
      <span class="broadcast-node"><NIcon :component="RadioOutline" /></span>
    </div>
    <div class="broadcast-content">
      <header class="broadcast-header">
        <div>
          <strong>{{ broadcast.senderId }}</strong>
          <span class="broadcast-id">{{ broadcast.id }}</span>
        </div>
        <time><NIcon :component="TimeOutline" />{{ formatTimestamp(broadcast.timestamp) }}</time>
      </header>

      <p v-if="shownText" class="message-body">{{ shownText }}</p>
      <div v-else class="blank-transmission">
        <span class="noise-line" /><span class="noise-line short" />
        <span>No readable payload recovered</span>
      </div>

      <div class="metadata-row" aria-label="Broadcast metadata">
        <NTag size="small" :bordered="false">
          <template #icon><NIcon :component="LocationOutline" /></template>
          {{ broadcast.location }}
        </NTag>
        <NTag size="small" :bordered="false">{{ titleCase(broadcast.type) }}</NTag>
        <NTag size="small" :type="signalType" :bordered="false">
          <template #icon><NIcon :component="CellularOutline" /></template>
          {{ broadcast.signalStrength ?? '—' }}{{ broadcast.signalStrength === null ? '' : '%' }} signal
        </NTag>
        <NTag size="small" :type="checkType" :bordered="false">
          <template #icon><NIcon :component="CheckmarkCircleOutline" /></template>
          {{ titleCase(broadcast.crossCheckStatus) }}
        </NTag>
        <NTag
          size="small"
          :type="broadcast.encryptionStatus === 'encrypted' ? 'warning' : 'default'"
          :bordered="false"
        >
          <template #icon>
            <NIcon :component="broadcast.encryptionStatus === 'encrypted' ? LockClosedOutline : LockOpenOutline" />
          </template>
          {{ broadcast.encryptionStatus === 'encrypted' ? 'Encrypted' : 'Unencrypted' }}
        </NTag>
      </div>
    </div>
  </article>
</template>

<style scoped>
.broadcast-item {
  display: grid;
  padding: 18px 0;
  border-bottom: 1px solid var(--border);
  grid-template-columns: 30px minmax(0, 1fr);
}

.broadcast-item:first-child { padding-top: 4px; }
.broadcast-rail { position: relative; }
.broadcast-rail::after { position: absolute; top: 25px; bottom: -19px; left: 11px; width: 1px; background: var(--border); content: ''; }
.broadcast-item:last-child .broadcast-rail::after { display: none; }
.broadcast-node { display: grid; width: 23px; height: 23px; border: 1px solid var(--accent); color: var(--accent); background: var(--surface); place-items: center; }
.encrypted .broadcast-node { border-color: var(--secure); color: var(--secure); }

.broadcast-header { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.broadcast-header strong { color: var(--text-strong); font-size: 0.82rem; }
.broadcast-id { margin-left: 8px; color: var(--text-faint); font: 0.68rem ui-monospace, monospace; }
.broadcast-header time { display: inline-flex; color: var(--text-faint); font-size: 0.72rem; align-items: center; gap: 5px; white-space: nowrap; }
.message-body { max-width: 780px; margin: 10px 0 12px; color: var(--text); font-size: 0.96rem; line-height: 1.58; overflow-wrap: anywhere; }
.metadata-row { display: flex; flex-wrap: wrap; gap: 6px; }
.blank-transmission { display: grid; max-width: 430px; margin: 12px 0; color: var(--text-faint); font-size: 0.76rem; gap: 5px; }
.noise-line { display: block; width: 75%; height: 3px; background: repeating-linear-gradient(90deg, var(--text-faint) 0 3px, transparent 3px 7px); opacity: 0.45; }
.noise-line.short { width: 45%; }

@media (max-width: 640px) {
  .broadcast-item { grid-template-columns: 24px minmax(0, 1fr); }
  .broadcast-header { align-items: flex-start; flex-direction: column; gap: 3px; }
  .broadcast-rail::after { left: 9px; }
  .broadcast-node { width: 19px; height: 19px; }
}
</style>
