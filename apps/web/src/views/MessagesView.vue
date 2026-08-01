<script setup lang="ts">
import { LockClosedOutline, LockOpenOutline, SendOutline } from '@vicons/ionicons5';
import { computed, nextTick, reactive, ref, watch } from 'vue';
import {
  NAlert,
  NButton,
  NCard,
  NEmpty,
  NIcon,
  NInput,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpin,
} from 'naive-ui';
import { broadcastTypes, type Broadcast, type BroadcastType, type StationSummary } from '@vts/common';
import { fetchStationBroadcasts } from '../api';
import BroadcastItem from '../components/BroadcastItem.vue';
import type { DisplayMode } from '../composables/useSessionPreferences';
import { titleCase } from '../utils/format';

const props = defineProps<{
  stations: StationSummary[];
  selectedStationId?: string;
  analysisTimestamp?: string;
  displayMode: DisplayMode;
  secretPhrase: string;
  currentStationId?: string;
}>();

const emit = defineEmits<{
  selectStation: [senderId: string];
  stationUpdated: [station: StationSummary];
  openSettings: [];
}>();

const remoteBroadcasts = reactive<Record<string, Broadcast[]>>({});
const localBroadcasts = reactive<Record<string, Broadcast[]>>({});
const localStubIds = new Set<string>();
const loading = ref(false);
const error = ref<string>();
const draft = ref('');
const selectedType = ref<BroadcastType>('routine_check');
const sendMode = ref<'secure' | 'plain'>('secure');
const historyScroll = ref<HTMLElement>();

const selectedStation = computed(() => props.stations.find((station) => station.senderId === props.selectedStationId));
const broadcasts = computed(() => {
  if (!props.selectedStationId) return [];
  return [...(remoteBroadcasts[props.selectedStationId] ?? []), ...(localBroadcasts[props.selectedStationId] ?? [])]
    .sort((a, b) => new Date(a.timestamp.replace(' ', 'T')).valueOf() - new Date(b.timestamp.replace(' ', 'T')).valueOf());
});
const typeOptions = broadcastTypes.map((value) => ({ label: titleCase(value), value }));
const phraseValid = computed(() => props.secretPhrase.length >= 16);
const canUseComposer = computed(() => Boolean(selectedStation.value && selectedStation.value.senderId === props.currentStationId));
const canSend = computed(() => canUseComposer.value && draft.value.trim().length > 0 && (sendMode.value === 'plain' || phraseValid.value));

function scrollToLatest() {
  void nextTick(() => {
    historyScroll.value?.scrollTo({ top: historyScroll.value.scrollHeight, behavior: 'smooth' });
  });
}

async function loadBroadcasts(senderId?: string) {
  if (!senderId || remoteBroadcasts[senderId]) {
    scrollToLatest();
    return;
  }
  loading.value = true;
  error.value = undefined;
  try {
    const response = await fetchStationBroadcasts(senderId);
    remoteBroadcasts[senderId] = response.broadcasts;
    scrollToLatest();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Could not load station broadcasts.';
  } finally {
    loading.value = false;
  }
}

watch(() => props.selectedStationId, loadBroadcasts, { immediate: true });

function hashText(value: string): number {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function createCarrier(senderId: string, message: string, type: BroadcastType): string {
  const openings = [
    'Visibility is holding steady along the eastern path.',
    'The morning patrol completed its usual circuit without delay.',
    'Conditions remain calm around the old garden structures.',
    'Supplies arrived with the scheduled relay and have been inventoried.',
    'A light wind has cleared the lower approach since the last check-in.',
  ];
  const details = [
    'We counted the markers twice and everything appears to be in order.',
    'The next team can proceed on the regular timetable.',
    'No unusual movement was observed during the latest watch.',
    'The crew is rotating duties before the next routine inspection.',
    'A second observer confirmed the report from the ridge.',
  ];
  const closings = [
    'We will send another update after the next round.',
    'No assistance is needed at this time.',
    'The station remains available for further instructions.',
    'We will keep the channel open for the evening relay.',
    'The team is continuing with normal operations.',
  ];
  const seed = hashText(`${props.secretPhrase}\u0000${senderId}\u0000${type}\u0000${message}`);
  return `${openings[seed % openings.length]} ${details[Math.floor(seed / 7) % details.length]} ${closings[Math.floor(seed / 31) % closings.length]}`;
}

function nextTimestamp(): string {
  const timestamps = [props.analysisTimestamp, ...broadcasts.value.map((item) => item.timestamp)].filter(Boolean) as string[];
  const latest = Math.max(...timestamps.map((value) => new Date(value.replace(' ', 'T')).valueOf()).filter(Number.isFinite));
  return new Date((Number.isFinite(latest) ? latest : Date.now()) + 60_000).toISOString().slice(0, 16).replace('T', ' ');
}

function sendBroadcast() {
  const station = selectedStation.value;
  const message = draft.value.trim();
  if (!station || !message || !canSend.value) return;

  const secure = sendMode.value === 'secure';
  const id = `LOCAL-${station.senderId}-${(localBroadcasts[station.senderId]?.length ?? 0) + 1}`;
  const broadcast: Broadcast = {
    id,
    timestamp: nextTimestamp(),
    senderId: station.senderId,
    location: station.location,
    type: selectedType.value,
    messageText: message,
    carrierText: secure ? createCarrier(station.senderId, message, selectedType.value) : null,
    encryptionStatus: secure ? 'encrypted' : 'plain',
    signalStrength: 100,
    crossCheckStatus: 'not_checked',
  };

  (localBroadcasts[station.senderId] ??= []).push(broadcast);
  if (secure) localStubIds.add(id);
  emit('stationUpdated', {
    ...station,
    lastBroadcastAt: broadcast.timestamp,
    lastMessagePreview: message,
    broadcastCount: station.broadcastCount + 1,
  });
  draft.value = '';
  scrollToLatest();
}
</script>

<template>
  <div class="messages-view">
    <NAlert v-if="error" type="error" closable @close="error = undefined">{{ error }}</NAlert>

    <NCard v-if="selectedStation" class="conversation-card" content-style="height: 100%; padding: 0;">
      <NSpin :show="loading" class="history-spinner">
        <div ref="historyScroll" class="history-scroll">
          <NEmpty v-if="!loading && !broadcasts.length" description="No broadcasts recorded for this station." />
          <div v-else class="broadcast-feed">
            <BroadcastItem
              v-for="broadcast in broadcasts"
              :key="broadcast.id"
              :broadcast="broadcast"
              :display-mode="displayMode"
              :stub="localStubIds.has(broadcast.id)"
            />
          </div>
        </div>
      </NSpin>
    </NCard>

    <NEmpty v-else description="Choose a station to inspect its broadcasts." />

    <NAlert v-if="selectedStation && !canUseComposer" type="info">
      Signed in as {{ currentStationId }}. Open that station to compose local session broadcasts.
    </NAlert>

    <section v-if="selectedStation && canUseComposer" class="composer" aria-label="Compose broadcast">
      <div class="composer-toolbar">
        <NSelect v-model:value="selectedType" size="small" :options="typeOptions" aria-label="Broadcast type" />
        <NRadioGroup v-model:value="sendMode" size="small" aria-label="Security mode">
          <NRadioButton value="secure"><span class="radio-label"><NIcon :component="LockClosedOutline" />Secure</span></NRadioButton>
          <NRadioButton value="plain"><span class="radio-label"><NIcon :component="LockOpenOutline" />Plain</span></NRadioButton>
        </NRadioGroup>
        <span class="composer-note">Local session only</span>
      </div>
      <div class="composer-input">
        <NInput
          v-model:value="draft"
          type="textarea"
          :autosize="{ minRows: 1, maxRows: 4 }"
          maxlength="1200"
          show-count
          placeholder="Write a broadcast…"
          @keydown.ctrl.enter.prevent="sendBroadcast"
          @keydown.meta.enter.prevent="sendBroadcast"
        />
        <NButton type="primary" circle size="large" :disabled="!canSend" title="Send broadcast" @click="sendBroadcast">
          <template #icon><NIcon :component="SendOutline" /></template>
        </NButton>
      </div>
      <button v-if="sendMode === 'secure' && !phraseValid" type="button" class="phrase-warning" @click="emit('openSettings')">
        Secure sends need a 16-character secret phrase. Open Settings.
      </button>
      <span v-else-if="sendMode === 'secure'" class="secure-note">Secure mode currently generates a deterministic carrier stub.</span>
    </section>
  </div>
</template>

<style scoped>
.messages-view { display: flex; height: 100%; min-height: 0; flex-direction: column; gap: 12px; overflow: hidden; }
.conversation-card { min-height: 0; flex: 1 1 auto; overflow: hidden; }
.history-spinner { height: 100%; min-height: 0; }
.history-spinner :deep(.n-spin-container), .history-spinner :deep(.n-spin-content) { height: 100%; min-height: 0; }
.history-scroll { height: 100%; min-height: 0; padding: 14px 16px; overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; }
.broadcast-feed { min-height: 200px; }
.composer { z-index: 4; padding: 11px 12px; flex: 0 0 auto; border: 1px solid var(--border-strong); background: var(--surface-raised); box-shadow: 0 -10px 30px rgba(0, 0, 0, 0.2); }
.composer-toolbar { display: grid; margin-bottom: 8px; grid-template-columns: minmax(150px, 220px) auto 1fr; gap: 10px; align-items: center; }
.composer-note { color: var(--text-faint); font-size: 0.68rem; text-align: right; text-transform: uppercase; }
.composer-input { display: grid; grid-template-columns: minmax(0, 1fr) 42px; gap: 10px; align-items: center; }
.radio-label { display: inline-flex; align-items: center; gap: 5px; }
.phrase-warning { margin: 7px 0 0; padding: 0; border: 0; color: var(--danger); background: none; cursor: pointer; font: inherit; font-size: 0.72rem; text-align: left; }
.secure-note { display: block; margin-top: 7px; color: var(--text-faint); font-size: 0.7rem; }

@media (max-width: 780px) {
  .composer-toolbar { grid-template-columns: minmax(130px, 1fr) auto; }
  .composer-note { display: none; }
}
@media (max-width: 640px) {
  .composer-toolbar { grid-template-columns: 1fr; }
  .history-scroll { padding: 12px; }
}
</style>
