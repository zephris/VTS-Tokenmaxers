<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue';
import {
  NAlert, NButton, NCard, NDataTable, NEmpty, NInput, NPagination, NSelect,
  NSpace, NSpin, NStatistic, NTag, type DataTableColumns,
} from 'naive-ui';
import {
  broadcastTypes, type Broadcast, type BroadcastFilters, type BroadcastsResponse,
  type CrossCheckStatus, type DashboardResponse, type IncidentSummaryResponse,
  type OutpostDetailResponse, type OutpostSummary,
} from '@vts/common';
import SilenceChart from '../components/SilenceChart.vue';
import { fetchBroadcasts, fetchIncidentSummary, fetchOutpost } from '../api';
import { formatTimestamp, titleCase } from '../utils/format';

const props = defineProps<{ dashboard: DashboardResponse }>();
const selectedSender = ref(props.dashboard.outposts[0]?.senderId);
const incidentSummary = ref<IncidentSummaryResponse>();
const summaryLoading = ref(false);
const summaryError = ref<string>();
const detail = ref<OutpostDetailResponse>();
const detailLoading = ref(false);
const detailError = ref<string>();
const broadcastResult = ref<BroadcastsResponse>();
const broadcastsLoading = ref(false);
const broadcastsError = ref<string>();
const filters = reactive<BroadcastFilters>({ page: 1, limit: 10 });

const tagType: Record<OutpostSummary['riskLevel'], 'success' | 'warning' | 'error'> = {
  low: 'success', medium: 'warning', high: 'error', critical: 'error',
};
const checkTagType: Record<CrossCheckStatus, 'success' | 'warning' | 'error' | 'default'> = {
  verified: 'success', disputed: 'error', unconfirmed: 'warning', not_checked: 'default',
};
const outpostColumns: DataTableColumns<OutpostSummary> = [
  { title: 'Outpost', key: 'senderId', minWidth: 150 },
  { title: 'Location', key: 'location', minWidth: 140 },
  { title: 'Silent', key: 'hoursSilent', minWidth: 90, render: (row) => `${row.hoursSilent.toFixed(1)}h`, sorter: (a, b) => a.hoursSilent - b.hoursSilent },
  { title: 'Expected cadence', key: 'expectedCadenceHours', minWidth: 130, render: (row) => `${(row.expectedCadenceHours ?? 24).toFixed(1)}h` },
  { title: 'Silence ratio', key: 'silenceRatio', minWidth: 110, render: (row) => `${(row.silenceRatio ?? row.hoursSilent / 24).toFixed(1)}×` },
  { title: 'Emergencies', key: 'unresolvedEmergencyCount', minWidth: 105, render: (row) => row.unresolvedEmergencyCount ?? 0 },
  { title: 'Contradictions', key: 'contradictionCount', minWidth: 110, render: (row) => row.contradictionCount ?? 0 },
  { title: 'Risk', key: 'riskLevel', minWidth: 90, render: (row) => h(NTag, { type: tagType[row.riskLevel], bordered: false, size: 'small' }, { default: () => row.riskLevel }) },
];
const broadcastColumns: DataTableColumns<Broadcast> = [
  { title: 'Time', key: 'timestamp', minWidth: 145, render: (row) => formatTimestamp(row.timestamp) },
  { title: 'Sender', key: 'senderId', minWidth: 135 },
  { title: 'Location', key: 'location', minWidth: 130 },
  { title: 'Type', key: 'type', minWidth: 120, render: (row) => titleCase(row.type) },
  { title: 'Cross-check', key: 'crossCheckStatus', minWidth: 115, render: (row) => h(NTag, { type: checkTagType[row.crossCheckStatus], bordered: false, size: 'small' }, { default: () => titleCase(row.crossCheckStatus) }) },
  { title: 'Message', key: 'messageText', minWidth: 300, ellipsis: { tooltip: true }, render: (row) => row.messageText || row.carrierText || 'No readable payload' },
];

const senderOptions = computed(() => props.dashboard.outposts.map((item) => ({ label: item.senderId, value: item.senderId })));
const typeOptions = broadcastTypes.map((value) => ({ label: titleCase(value), value }));
const crossCheckOptions: Array<{ label: string; value: CrossCheckStatus }> = [
  { label: 'Verified', value: 'verified' }, { label: 'Disputed', value: 'disputed' },
  { label: 'Unconfirmed', value: 'unconfirmed' }, { label: 'Not checked', value: 'not_checked' },
];
const criticalCount = computed(() => props.dashboard.outposts.filter((item) => item.riskLevel === 'critical').length);
const unresolvedEmergencyCount = computed(() => props.dashboard.outposts.reduce((sum, item) => sum + (item.unresolvedEmergencyCount ?? 0), 0));
const contradictionCount = computed(() => props.dashboard.outposts.reduce((sum, item) => sum + (item.contradictionCount ?? 0), 0));

async function generateSummary() {
  if (!selectedSender.value) return;
  summaryLoading.value = true;
  summaryError.value = undefined;
  try { incidentSummary.value = await fetchIncidentSummary(selectedSender.value); }
  catch (caught) { summaryError.value = caught instanceof Error ? caught.message : 'Could not generate summary.'; }
  finally { summaryLoading.value = false; }
}

async function loadOutpost(senderId?: string) {
  if (!senderId) return;
  detailLoading.value = true;
  detailError.value = undefined;
  detail.value = undefined;
  try { detail.value = await fetchOutpost(senderId); }
  catch (caught) { detailError.value = caught instanceof Error ? caught.message : 'Could not load outpost evidence.'; }
  finally { detailLoading.value = false; }
}

async function loadBroadcasts() {
  broadcastsLoading.value = true;
  broadcastsError.value = undefined;
  try { broadcastResult.value = await fetchBroadcasts(filters); }
  catch (caught) { broadcastsError.value = caught instanceof Error ? caught.message : 'Could not search broadcasts.'; }
  finally { broadcastsLoading.value = false; }
}

function searchBroadcasts() { filters.page = 1; void loadBroadcasts(); }
function resetFilters() {
  filters.senderId = undefined; filters.location = undefined; filters.type = undefined;
  filters.crossCheckStatus = undefined; filters.q = undefined; filters.page = 1;
  void loadBroadcasts();
}
function changePage(page: number) { filters.page = page; void loadBroadcasts(); }
function outpostRowProps(row: OutpostSummary) {
  return {
    tabindex: 0, style: 'cursor: pointer', onClick: () => { selectedSender.value = row.senderId; },
    onKeydown: (event: KeyboardEvent) => {
      if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); selectedSender.value = row.senderId; }
    },
  };
}

watch(selectedSender, (senderId) => {
  incidentSummary.value = undefined;
  summaryError.value = undefined;
  void loadOutpost(senderId);
}, { immediate: true });
onMounted(loadBroadcasts);
</script>

<template>
  <div class="view-stack">
    <header class="view-header"><div><span class="eyebrow">Operational intelligence</span><h1>Network monitoring</h1><p>Inspect silence against expected cadence, trace risk evidence, and search the broadcast record.</p></div></header>

    <section class="metric-grid four">
      <NCard><NStatistic label="Tracked outposts" :value="dashboard.outposts.length" /></NCard>
      <NCard><NStatistic label="Critical alerts" :value="criticalCount" /></NCard>
      <NCard><NStatistic label="Unresolved emergencies" :value="unresolvedEmergencyCount" /></NCard>
      <NCard><NStatistic label="Contradictions" :value="contradictionCount" /></NCard>
    </section>

    <section class="analysis-grid">
      <NCard title="Silence risk"><SilenceChart :outposts="dashboard.outposts" /></NCard>
      <NCard title="Incident brief preview">
        <NSpace vertical :size="14">
          <NSelect v-model:value="selectedSender" :options="senderOptions" placeholder="Choose an outpost" />
          <NButton type="primary" :loading="summaryLoading" :disabled="!selectedSender" @click="generateSummary">Analyse evidence</NButton>
          <NAlert v-if="summaryError" type="error">{{ summaryError }}</NAlert>
          <NAlert v-else-if="incidentSummary" :type="incidentSummary.source === 'ai' ? 'info' : 'warning'">
            <template #header>{{ incidentSummary.source === 'ai' ? 'AI-generated brief' : 'Deterministic preview' }}</template>
            {{ incidentSummary.summary }}
            <p class="evidence">Evidence: {{ incidentSummary.evidenceIds.join(', ') || 'none' }}</p>
          </NAlert>
          <NEmpty v-else description="Choose an outpost to analyse its evidence." />
        </NSpace>
      </NCard>
    </section>

    <NCard title="Outpost watchlist">
      <template #header-extra><span class="section-note">Select a row to inspect its evidence</span></template>
      <NDataTable :columns="outpostColumns" :data="dashboard.outposts" :row-key="(row: OutpostSummary) => row.senderId" :row-props="outpostRowProps" :scroll-x="1130" />
    </NCard>

    <NCard :title="selectedSender ? `Evidence · ${selectedSender}` : 'Outpost evidence'">
      <NSpin :show="detailLoading">
        <NAlert v-if="detailError" type="error">{{ detailError }}</NAlert>
        <NEmpty v-else-if="!detail" description="Select an outpost to load its risk evidence." />
        <div v-else class="detail-stack">
          <div class="detail-metrics">
            <div><span>Risk</span><strong>{{ titleCase(detail.outpost.riskLevel) }}</strong></div>
            <div><span>Silence</span><strong>{{ detail.outpost.hoursSilent.toFixed(1) }}h</strong></div>
            <div><span>Expected cadence</span><strong>{{ detail.outpost.expectedCadenceHours.toFixed(1) }}h</strong></div>
            <div><span>Silence ratio</span><strong>{{ detail.outpost.silenceRatio.toFixed(1) }}×</strong></div>
          </div>
          <div><h3>Why this risk level</h3>
            <NEmpty v-if="!detail.outpost.riskReasons.length" size="small" description="No elevated risk reasons." />
            <ul v-else class="reason-list">
              <li v-for="reason in detail.outpost.riskReasons" :key="`${reason.code}-${reason.evidenceIds.join('-')}`">
                <NTag size="small" :type="tagType[reason.severity]" :bordered="false">{{ titleCase(reason.severity) }}</NTag>
                <div><strong>{{ titleCase(reason.code) }}</strong><p>{{ reason.message }}</p><span>Evidence: {{ reason.evidenceIds.join(', ') || 'derived metric' }}</span></div>
              </li>
            </ul>
          </div>
          <div><h3>Evidence timeline</h3>
            <NEmpty v-if="!detail.evidenceTimeline.length" size="small" description="No evidence broadcasts." />
            <ol v-else class="evidence-timeline">
              <li v-for="item in detail.evidenceTimeline" :key="item.id">
                <time>{{ formatTimestamp(item.timestamp) }}</time>
                <div><strong>{{ item.id }} · {{ titleCase(item.type) }}</strong><p>{{ item.messageText || 'No readable payload' }}</p></div>
                <NTag size="small" :type="checkTagType[item.crossCheckStatus]" :bordered="false">{{ titleCase(item.crossCheckStatus) }}</NTag>
              </li>
            </ol>
          </div>
        </div>
      </NSpin>
    </NCard>

    <NCard title="Broadcast search">
      <div class="filter-grid">
        <NInput v-model:value="filters.q" clearable placeholder="Search message text…" aria-label="Search message text" @keyup.enter="searchBroadcasts" />
        <NSelect v-model:value="filters.senderId" clearable filterable :options="senderOptions" placeholder="Any sender" />
        <NInput v-model:value="filters.location" clearable placeholder="Any location" aria-label="Filter by location" @keyup.enter="searchBroadcasts" />
        <NSelect v-model:value="filters.type" clearable :options="typeOptions" placeholder="Any type" />
        <NSelect v-model:value="filters.crossCheckStatus" clearable :options="crossCheckOptions" placeholder="Any cross-check" />
        <div class="filter-actions"><NButton type="primary" @click="searchBroadcasts">Search</NButton><NButton @click="resetFilters">Reset</NButton></div>
      </div>
      <NAlert v-if="broadcastsError" type="error" class="broadcast-error">{{ broadcastsError }}</NAlert>
      <NDataTable :loading="broadcastsLoading" :columns="broadcastColumns" :data="broadcastResult?.broadcasts ?? []" :row-key="(row: Broadcast) => row.id" :scroll-x="1050" />
      <div v-if="broadcastResult?.pagination.total" class="pagination-row">
        <span>{{ broadcastResult.pagination.total }} matching broadcasts</span>
        <NPagination :page="broadcastResult.pagination.page" :page-count="broadcastResult.pagination.totalPages" @update:page="changePage" />
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.analysis-grid { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(300px, 0.7fr); gap: 14px; }
.evidence, .section-note { margin: 10px 0 0; color: var(--text-faint); font-size: 0.72rem; }
.detail-stack { display: grid; gap: 22px; }
.detail-stack h3 { margin: 0 0 10px; color: var(--text-strong); font-size: 0.84rem; }
.detail-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
.detail-metrics div { padding: 12px; border: 1px solid var(--border); background: var(--surface-raised); }
.detail-metrics span, .detail-metrics strong { display: block; }
.detail-metrics span { color: var(--text-faint); font-size: 0.68rem; text-transform: uppercase; }
.detail-metrics strong { margin-top: 4px; color: var(--text-strong); }
.reason-list, .evidence-timeline { padding: 0; margin: 0; list-style: none; }
.reason-list li { display: grid; padding: 10px 0; border-bottom: 1px solid var(--border); grid-template-columns: 72px minmax(0, 1fr); gap: 10px; }
.reason-list strong { color: var(--text-strong); font-size: 0.8rem; }
.reason-list p { margin: 3px 0; color: var(--text); font-size: 0.78rem; }
.reason-list span { color: var(--text-faint); font-size: 0.68rem; }
.evidence-timeline li { display: grid; padding: 10px 0; border-bottom: 1px solid var(--border); grid-template-columns: 150px minmax(0, 1fr) auto; gap: 12px; align-items: start; }
.evidence-timeline time { color: var(--text-faint); font-size: 0.72rem; }
.evidence-timeline strong { color: var(--text-strong); font-size: 0.78rem; }
.evidence-timeline p { margin: 4px 0 0; color: var(--text-muted); font-size: 0.78rem; line-height: 1.4; }
.filter-grid { display: grid; margin-bottom: 14px; grid-template-columns: repeat(3, minmax(160px, 1fr)); gap: 10px; }
.filter-actions { display: flex; gap: 8px; }
.broadcast-error { margin-bottom: 12px; }
.pagination-row { display: flex; padding-top: 14px; color: var(--text-faint); font-size: 0.72rem; align-items: center; justify-content: space-between; gap: 12px; }
@media (max-width: 980px) { .analysis-grid { grid-template-columns: 1fr; } .detail-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } .filter-grid { grid-template-columns: repeat(2, minmax(150px, 1fr)); } }
@media (max-width: 620px) { .detail-metrics, .filter-grid { grid-template-columns: 1fr; } .evidence-timeline li { grid-template-columns: 1fr; gap: 4px; } .pagination-row { align-items: flex-start; flex-direction: column; } }
</style>
