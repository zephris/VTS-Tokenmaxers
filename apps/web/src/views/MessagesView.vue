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
import {
  broadcastTypes,
  type Broadcast,
  type BroadcastType,
  type StationSummary,
  type SteganographyRecord,
} from '@vts/common';
import { encodeSteganography, fetchStationBroadcasts, fetchSteganographyConversation } from '../api';
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
  modelConfigured: boolean;
}>();

const emit = defineEmits<{
  selectStation: [senderId: string];
  stationUpdated: [station: StationSummary];
  openSettings: [];
}>();

const remoteBroadcasts = reactive<Record<string, Broadcast[]>>({});
const localBroadcasts = reactive<Record<string, Broadcast[]>>({});
const secureRecords = reactive<Record<string, SteganographyRecord[]>>({});
const timelineBroadcasts = reactive<Record<string, Broadcast[]>>({});
const sessionPlaintexts = reactive<Record<string, string>>({});
const loading = ref(false);
const sending = ref(false);
const error = ref<string>();
const draft = ref('');
const selectedType = ref<BroadcastType>('routine_check');
const sendMode = ref<'secure' | 'plain'>('secure');
const historyScroll = ref<HTMLElement>();

const selectedStation = computed(() => props.stations.find((station) => station.senderId === props.selectedStationId));
const broadcasts = computed(() => {
  if (!props.selectedStationId) return [];
  return timelineBroadcasts[props.selectedStationId] ?? [];
});
const typeOptions = broadcastTypes.map((value) => ({ label: titleCase(value), value }));
const phraseValid = computed(() => props.secretPhrase.length >= 16);
const canUseComposer = computed(() => Boolean(selectedStation.value && selectedStation.value.senderId === props.currentStationId));
const canSend = computed(() => canUseComposer.value
  && !sending.value
  && draft.value.trim().length > 0
  && (sendMode.value === 'plain' || (phraseValid.value && props.modelConfigured)));
const composerNote = computed(() => {
  if (sendMode.value === 'plain') return 'Local session only';
  return props.modelConfigured ? 'GPT-2 carrier generation' : 'Model unavailable';
});

function conversationId(senderId: string): string {
  return `station-broadcasts:${senderId}`;
}

function secureRecordId(senderId: string, record: SteganographyRecord): string {
  return `SECURE-${senderId}-${record.index}`;
}

function recordToBroadcast(record: SteganographyRecord, station: StationSummary): Broadcast {
  const id = secureRecordId(station.senderId, record);
  return {
    id,
    timestamp: record.createdAt ?? props.analysisTimestamp ?? new Date().toISOString(),
    senderId: record.from,
    location: station.location,
    type: record.broadcastType ?? 'routine_check',
    messageText: sessionPlaintexts[id] ?? null,
    carrierText: record.carrierText,
    encryptionStatus: 'encrypted',
    signalStrength: 100,
    crossCheckStatus: 'not_checked',
  };
}

function timestampValue(broadcast: Broadcast): number {
  const value = new Date(broadcast.timestamp.replace(' ', 'T')).valueOf();
  return Number.isFinite(value) ? value : 0;
}

function mergeTimeline(senderId: string, incoming: Broadcast[]) {
  const timeline = timelineBroadcasts[senderId];
  if (!timeline) {
    timelineBroadcasts[senderId] = [...incoming].sort((a, b) => timestampValue(a) - timestampValue(b));
    return;
  }

  const existingIds = new Set(timeline.map((broadcast) => broadcast.id));
  for (const broadcast of incoming) {
    if (existingIds.has(broadcast.id)) continue;
    timeline.push(broadcast);
    existingIds.add(broadcast.id);
  }
}

function appendBroadcast(senderId: string, broadcast: Broadcast) {
  mergeTimeline(senderId, [broadcast]);
}

function scrollToLatest() {
  void nextTick(() => {
    historyScroll.value?.scrollTo({ top: historyScroll.value.scrollHeight, behavior: 'smooth' });
  });
}

async function loadBroadcasts(senderId?: string) {
  if (!senderId) {
    scrollToLatest();
    return;
  }
  loading.value = true;
  error.value = undefined;
  try {
    const datasetRequest = remoteBroadcasts[senderId]
      ? Promise.resolve(undefined)
      : fetchStationBroadcasts(senderId);
    const [dataset, transcript] = await Promise.all([
      datasetRequest,
      fetchSteganographyConversation(conversationId(senderId), senderId),
    ]);
    if (dataset) remoteBroadcasts[senderId] = dataset.broadcasts;
    secureRecords[senderId] = transcript.records;
    const station = props.stations.find((candidate) => candidate.senderId === senderId);
    if (station) {
      mergeTimeline(senderId, [
        ...(remoteBroadcasts[senderId] ?? []),
        ...transcript.records.map((record) => recordToBroadcast(record, station)),
        ...(localBroadcasts[senderId] ?? []),
      ]);
    }
    scrollToLatest();
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Could not load station broadcasts.';
  } finally {
    loading.value = false;
  }
}

watch(() => props.selectedStationId, loadBroadcasts, { immediate: true });

function nextTimestamp(): string {
  const timestamps = [props.analysisTimestamp, ...broadcasts.value.map((item) => item.timestamp)].filter(Boolean) as string[];
  const latest = Math.max(...timestamps.map((value) => new Date(value.replace(' ', 'T')).valueOf()).filter(Number.isFinite));
  return new Date((Number.isFinite(latest) ? latest : Date.now()) + 60_000).toISOString().slice(0, 16).replace('T', ' ');
}

async function sendBroadcast() {
  const station = selectedStation.value;
  const message = draft.value.trim();
  if (!station || !message || !canSend.value) return;

  const secure = sendMode.value === 'secure';
  if (secure) {
    sending.value = true;
    error.value = undefined;
    try {
      const response = await encodeSteganography({
        conversationId: conversationId(station.senderId),
        stationId: station.senderId,
        sender: station.senderId,
        secretPhrase: props.secretPhrase,
        plaintext: message,
        broadcastType: selectedType.value,
      });
      const id = secureRecordId(station.senderId, response.record);
      sessionPlaintexts[id] = message;
      secureRecords[station.senderId] = response.records;
      appendBroadcast(station.senderId, recordToBroadcast(response.record, station));
      const timestamp = response.record.createdAt ?? new Date().toISOString();
      emit('stationUpdated', {
        ...station,
        lastBroadcastAt: timestamp,
        lastMessagePreview: message,
        broadcastCount: station.broadcastCount + 1,
      });
      draft.value = '';
      scrollToLatest();
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Secure carrier generation failed. Retry the send.';
    } finally {
      sending.value = false;
    }
    return;
  }

  const id = `LOCAL-${station.senderId}-${(localBroadcasts[station.senderId]?.length ?? 0) + 1}`;
  const broadcast: Broadcast = {
    id,
    timestamp: nextTimestamp(),
    senderId: station.senderId,
    location: station.location,
    type: selectedType.value,
    messageText: message,
    carrierText: null,
    encryptionStatus: 'plain',
    signalStrength: 100,
    crossCheckStatus: 'not_checked',
  };

  (localBroadcasts[station.senderId] ??= []).push(broadcast);
  appendBroadcast(station.senderId, broadcast);
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
        <span class="composer-note">{{ composerNote }}</span>
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
        <NButton type="primary" circle size="large" :disabled="!canSend" :loading="sending" title="Send broadcast" @click="sendBroadcast">
          <template #icon><NIcon :component="SendOutline" /></template>
        </NButton>
      </div>
      <button v-if="sendMode === 'secure' && !phraseValid" type="button" class="phrase-warning" @click="emit('openSettings')">
        Secure sends need a 16-character secret phrase. Open Settings.
      </button>
      <span v-else-if="sendMode === 'secure' && !modelConfigured" class="secure-note">
        Secure sends are unavailable until the GPT-2 model is enabled on the server.
      </span>
      <span v-else-if="sendMode === 'secure'" class="secure-note">
        Plaintext stays in this tab. Only the generated carrier is stored in the public transcript.
      </span>
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
