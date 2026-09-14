<script setup lang="ts">
import {
  CheckmarkCircleOutline,
  ClipboardOutline,
  KeyOutline,
  LockClosedOutline,
  RefreshOutline,
  SwapHorizontalOutline,
} from '@vicons/ionicons5';
import { computed, reactive, ref, watch } from 'vue';
import {
  NAlert,
  NButton,
  NCard,
  NCode,
  NEmpty,
  NIcon,
  NInput,
  NSelect,
  NSpin,
  NTag,
} from 'naive-ui';
import type {
  HealthResponse,
  StationSummary,
  SteganographyConversationResponse,
} from '@vts/common';
import {
  decodeSteganography,
  encodeSteganography,
  fetchSteganographyConversation,
} from '../api';

type Side = 'alpha' | 'bravo';

interface StationWorkspace {
  plaintext: string;
  generatedCarrier: string;
  incomingCarrier: string;
  incomingSender: string;
  recoveredPlaintext: string;
  conversation?: SteganographyConversationResponse;
  loadingTranscript: boolean;
  encoding: boolean;
  decoding: boolean;
  error?: string;
  copied: boolean;
}

const props = defineProps<{
  stations: StationSummary[];
  health?: HealthResponse;
  currentStationId?: string;
  secretPhrase: string;
}>();

const emit = defineEmits<{ openSettings: [] }>();
const conversationDraft = ref('silent-outposts-demo');
const conversationId = ref('silent-outposts-demo');
const selectedStations = reactive<Record<Side, string>>({ alpha: '', bravo: '' });
const workspace = reactive<Record<Side, StationWorkspace>>({
  alpha: createWorkspace(),
  bravo: createWorkspace(),
});

function createWorkspace(): StationWorkspace {
  return {
    plaintext: '',
    generatedCarrier: '',
    incomingCarrier: '',
    incomingSender: '',
    recoveredPlaintext: '',
    loadingTranscript: false,
    encoding: false,
    decoding: false,
    copied: false,
  };
}

const stationOptions = computed(() => props.stations.map((station) => ({
  label: `${station.senderId} · ${station.location}`,
  value: station.senderId,
})));
const phraseValid = computed(() => props.secretPhrase.length >= 16);
const configured = computed(() => props.health?.steganography.configured === true);
const modelLabel = computed(() => props.health?.steganography.modelFingerprint || 'Model unavailable');
const synced = computed(() => {
  const alphaCode = workspace.alpha.conversation?.syncCode;
  const bravoCode = workspace.bravo.conversation?.syncCode;
  return Boolean(alphaCode && bravoCode && alphaCode === bravoCode);
});

watch([() => props.stations, () => props.currentStationId], ([stations, currentStationId]) => {
  const ownStation = currentStationId && stations.some((station) => station.senderId === currentStationId)
    ? currentStationId
    : stations[0]?.senderId ?? 'station-alpha';
  const peerStation = stations.find((station) => station.senderId !== ownStation)?.senderId ?? ownStation;
  if (!selectedStations.alpha) selectedStations.alpha = ownStation;
  if (!selectedStations.bravo) selectedStations.bravo = peerStation;
  workspace.alpha.incomingSender ||= selectedStations.bravo;
  workspace.bravo.incomingSender ||= selectedStations.alpha;
}, { immediate: true });

watch(() => selectedStations.alpha, (value, previous) => {
  if (!workspace.bravo.incomingSender || workspace.bravo.incomingSender === previous) {
    workspace.bravo.incomingSender = value;
  }
});
watch(() => selectedStations.bravo, (value, previous) => {
  if (!workspace.alpha.incomingSender || workspace.alpha.incomingSender === previous) {
    workspace.alpha.incomingSender = value;
  }
});

function opposite(side: Side): Side {
  return side === 'alpha' ? 'bravo' : 'alpha';
}

function clearError(side: Side) {
  workspace[side].error = undefined;
}

async function refreshConversation(side: Side) {
  const state = workspace[side];
  const stationId = selectedStations[side];
  if (!configured.value || !stationId || !conversationId.value) return;
  state.loadingTranscript = true;
  clearError(side);
  try {
    state.conversation = await fetchSteganographyConversation(conversationId.value, stationId);
  } catch (caught) {
    state.error = caught instanceof Error ? caught.message : 'Could not load the public transcript.';
  } finally {
    state.loadingTranscript = false;
  }
}

async function loadConversation() {
  const nextId = conversationDraft.value.trim();
  if (!nextId) return;
  conversationId.value = nextId;
  workspace.alpha.conversation = undefined;
  workspace.bravo.conversation = undefined;
  await Promise.all([refreshConversation('alpha'), refreshConversation('bravo')]);
}

async function encode(side: Side) {
  const state = workspace[side];
  if (!configured.value || !phraseValid.value || !state.plaintext.length) return;
  state.encoding = true;
  state.generatedCarrier = '';
  clearError(side);
  try {
    const response = await encodeSteganography({
      conversationId: conversationId.value,
      stationId: selectedStations[side],
      sender: selectedStations[side],
      secretPhrase: props.secretPhrase,
      plaintext: state.plaintext,
    });
    state.generatedCarrier = response.carrierText;
    state.conversation = response;
  } catch (caught) {
    state.error = caught instanceof Error ? caught.message : 'Carrier generation failed.';
  } finally {
    state.encoding = false;
  }
}

async function decode(side: Side) {
  const state = workspace[side];
  if (!configured.value || !phraseValid.value || !state.incomingCarrier.length || !state.incomingSender) return;
  state.decoding = true;
  state.recoveredPlaintext = '';
  clearError(side);
  try {
    const response = await decodeSteganography({
      conversationId: conversationId.value,
      stationId: selectedStations[side],
      sender: state.incomingSender,
      secretPhrase: props.secretPhrase,
      carrierText: state.incomingCarrier,
    });
    state.recoveredPlaintext = response.plaintext;
    state.conversation = response;
  } catch (caught) {
    state.error = caught instanceof Error ? caught.message : 'Carrier decoding failed.';
  } finally {
    state.decoding = false;
  }
}

function transfer(side: Side) {
  const carrier = workspace[side].generatedCarrier;
  if (!carrier) return;
  const target = opposite(side);
  workspace[target].incomingCarrier = carrier;
  workspace[target].incomingSender = selectedStations[side];
  workspace[target].recoveredPlaintext = '';
  workspace[target].error = undefined;
}

async function copyCarrier(side: Side) {
  const carrier = workspace[side].generatedCarrier;
  if (!carrier) return;
  try {
    await navigator.clipboard.writeText(carrier);
    workspace[side].copied = true;
    window.setTimeout(() => { workspace[side].copied = false; }, 1800);
  } catch {
    workspace[side].error = 'Clipboard access was denied. Select and copy the carrier text manually.';
  }
}
</script>

<template>
  <div class="view-stack secure-view">
    <header class="view-header secure-header">
      <div>
        <span class="eyebrow">Experimental secure channel</span>
        <h1>Conversation steganography</h1>
        <p>Generate natural carrier text at one station, transfer it exactly, and recover the hidden message at the other.</p>
      </div>
      <div class="runtime-tags">
        <NTag :type="configured ? 'success' : 'error'" :bordered="false">
          {{ configured ? 'Model ready' : 'Model disabled' }}
        </NTag>
        <NTag :type="synced ? 'success' : 'warning'" :bordered="false">
          {{ synced ? 'Transcripts synchronized' : 'Transcripts differ' }}
        </NTag>
      </div>
    </header>

    <NAlert v-if="!configured" type="error" title="Secure messaging is unavailable">
      The server has not started its conversation-steganography model. Monitoring remains available, but encode and decode actions are disabled.
    </NAlert>
    <NAlert v-else-if="!phraseValid" type="warning" title="Secret phrase required">
      <div class="phrase-alert-content">
        <span>Save a phrase of at least 16 characters in Settings before using this channel.</span>
        <NButton size="small" @click="emit('openSettings')">Open Settings</NButton>
      </div>
    </NAlert>

    <NCard class="channel-control">
      <div class="conversation-control">
        <NInput
          v-model:value="conversationDraft"
          maxlength="200"
          placeholder="Conversation ID"
          aria-label="Conversation ID"
          @keyup.enter="loadConversation"
        />
        <NButton :disabled="!configured || !conversationDraft.trim()" @click="loadConversation">
          <template #icon><NIcon :component="RefreshOutline" /></template>
          Load channel
        </NButton>
      </div>
      <dl class="channel-status">
        <div><dt>Active conversation</dt><dd>{{ conversationId }}</dd></div>
        <div><dt>Algorithm</dt><dd>{{ health?.steganography.algorithm || 'Unavailable' }}</dd></div>
        <div><dt>Model fingerprint</dt><dd>{{ modelLabel }}</dd></div>
      </dl>
    </NCard>

    <div class="station-grid">
      <NCard v-for="side in (['alpha', 'bravo'] as Side[])" :key="side" class="station-workspace">
        <template #header>
          <div class="station-card-title">
            <span><NIcon :component="LockClosedOutline" />Station {{ side === 'alpha' ? 'A' : 'B' }}</span>
            <NTag size="small" :type="workspace[side].conversation ? 'success' : 'default'" :bordered="false">
              {{ workspace[side].conversation ? 'Transcript loaded' : 'No transcript' }}
            </NTag>
          </div>
        </template>

        <NSelect
          v-model:value="selectedStations[side]"
          :options="stationOptions"
          filterable
          :disabled="workspace[side].encoding || workspace[side].decoding"
          :aria-label="`Station ${side === 'alpha' ? 'A' : 'B'} identity`"
        />

        <NAlert v-if="workspace[side].error" type="error" closable class="station-alert" @close="clearError(side)">
          {{ workspace[side].error }}
        </NAlert>

        <section class="operation-block">
          <div class="operation-heading"><strong>1. Write hidden message</strong><span>Plaintext stays out of the public transcript</span></div>
          <NInput
            v-model:value="workspace[side].plaintext"
            type="textarea"
            :autosize="{ minRows: 3, maxRows: 6 }"
            maxlength="4000"
            show-count
            placeholder="Message to conceal…"
          />
          <NButton
            type="primary"
            block
            :loading="workspace[side].encoding"
            :disabled="!configured || !phraseValid || !workspace[side].plaintext.length"
            @click="encode(side)"
          >
            Generate carrier text
          </NButton>

          <div v-if="workspace[side].generatedCarrier" class="carrier-result">
            <span>Generated carrier — preserve every character</span>
            <NCode :code="workspace[side].generatedCarrier" word-wrap />
            <div class="carrier-actions">
              <NButton size="small" @click="copyCarrier(side)">
                <template #icon><NIcon :component="workspace[side].copied ? CheckmarkCircleOutline : ClipboardOutline" /></template>
                {{ workspace[side].copied ? 'Copied' : 'Copy exact text' }}
              </NButton>
              <NButton size="small" type="primary" secondary @click="transfer(side)">
                <template #icon><NIcon :component="SwapHorizontalOutline" /></template>
                Transfer to Station {{ side === 'alpha' ? 'B' : 'A' }}
              </NButton>
            </div>
          </div>
        </section>

        <section class="operation-block">
          <div class="operation-heading"><strong>2. Receive carrier</strong><span>Do not trim or edit this text</span></div>
          <NSelect
            v-model:value="workspace[side].incomingSender"
            :options="stationOptions"
            filterable
            placeholder="Original sender"
            aria-label="Original carrier sender"
          />
          <NInput
            v-model:value="workspace[side].incomingCarrier"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 8 }"
            placeholder="Paste the exact carrier text…"
          />
          <NButton
            block
            :loading="workspace[side].decoding"
            :disabled="!configured || !phraseValid || !workspace[side].incomingCarrier.length || !workspace[side].incomingSender"
            @click="decode(side)"
          >
            <template #icon><NIcon :component="KeyOutline" /></template>
            Recover hidden message
          </NButton>
          <NAlert v-if="workspace[side].recoveredPlaintext" type="success" title="Recovered plaintext">
            <span class="recovered-text">{{ workspace[side].recoveredPlaintext }}</span>
          </NAlert>
        </section>

        <section class="transcript-block">
          <div class="operation-heading">
            <strong>Public transcript</strong>
            <NButton text size="small" :disabled="!configured" @click="refreshConversation(side)">Refresh</NButton>
          </div>
          <NSpin :show="workspace[side].loadingTranscript">
            <NEmpty v-if="!workspace[side].conversation?.records.length" size="small" description="No public carrier records at this station." />
            <ol v-else class="transcript-list">
              <li v-for="record in workspace[side].conversation?.records" :key="record.index">
                <div><strong>#{{ record.index }} · {{ record.from }}</strong><span>sender seq {{ record.senderSequence }}</span></div>
                <p>{{ record.carrierText }}</p>
              </li>
            </ol>
          </NSpin>
          <div class="sync-code">
            <span>Sync code</span>
            <code>{{ workspace[side].conversation?.syncCode || '—' }}</code>
          </div>
        </section>
      </NCard>
    </div>

    <NAlert type="warning">
      This is experimental research software, not audited cryptography. Use TLS in deployment and never alter generated carrier text before decoding.
    </NAlert>
  </div>
</template>

<style scoped>
.secure-view { max-width: 1380px; }
.secure-header { min-height: auto; }
.runtime-tags { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 8px; }
.phrase-alert-content { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.channel-control { border-color: var(--border-strong); }
.conversation-control { display: grid; grid-template-columns: minmax(220px, 520px) auto; gap: 10px; }
.channel-status { display: grid; margin: 16px 0 0; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
.channel-status div { min-width: 0; padding-top: 10px; border-top: 1px solid var(--border); }
.channel-status dt { color: var(--text-faint); font-size: 0.68rem; text-transform: uppercase; }
.channel-status dd { overflow: hidden; margin: 4px 0 0; color: var(--text); font: 0.72rem ui-monospace, monospace; text-overflow: ellipsis; white-space: nowrap; }
.station-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; align-items: start; }
.station-workspace { min-width: 0; }
.station-card-title { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.station-card-title > span { display: inline-flex; align-items: center; gap: 7px; }
.station-alert { margin-top: 12px; }
.operation-block, .transcript-block { display: grid; padding-top: 18px; margin-top: 18px; border-top: 1px solid var(--border); gap: 10px; }
.operation-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }
.operation-heading strong { color: var(--text-strong); font-size: 0.82rem; }
.operation-heading span { color: var(--text-faint); font-size: 0.67rem; text-align: right; }
.carrier-result { display: grid; padding: 12px; border: 1px solid var(--border-strong); background: var(--surface-raised); gap: 10px; }
.carrier-result > span { color: var(--secure); font-size: 0.68rem; font-weight: 700; text-transform: uppercase; }
.carrier-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.recovered-text { white-space: pre-wrap; overflow-wrap: anywhere; }
.transcript-list { max-height: 310px; padding: 0; margin: 0; overflow-y: auto; list-style: none; }
.transcript-list li { padding: 10px 0; border-bottom: 1px solid var(--border); }
.transcript-list li > div { display: flex; color: var(--text-faint); font-size: 0.67rem; justify-content: space-between; gap: 8px; }
.transcript-list strong { color: var(--text-muted); }
.transcript-list p { margin: 6px 0 0; color: var(--text); font-size: 0.76rem; line-height: 1.5; overflow-wrap: anywhere; }
.sync-code { display: flex; padding-top: 10px; color: var(--text-faint); font-size: 0.68rem; justify-content: space-between; gap: 10px; }
.sync-code code { overflow: hidden; color: var(--accent); text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 1050px) {
  .station-grid { grid-template-columns: 1fr; }
}
@media (max-width: 680px) {
  .runtime-tags { justify-content: flex-start; }
  .secure-header { flex-direction: column; }
  .conversation-control, .channel-status { grid-template-columns: 1fr; }
  .operation-heading { align-items: flex-start; flex-direction: column; gap: 3px; }
  .operation-heading span { text-align: left; }
  .phrase-alert-content { align-items: flex-start; flex-direction: column; }
}
</style>
