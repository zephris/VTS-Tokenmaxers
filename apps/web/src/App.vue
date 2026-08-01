<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue';
import {
  NAlert,
  NButton,
  NCard,
  NConfigProvider,
  NDataTable,
  NEmpty,
  NLayout,
  NLayoutContent,
  NSelect,
  NSpace,
  NSpin,
  NStatistic,
  NTag,
  darkTheme,
  type DataTableColumns,
} from 'naive-ui';
import type { DashboardResponse, IncidentSummaryResponse, OutpostSummary } from '@vts/common';
import SilenceChart from './components/SilenceChart.vue';
import { fetchDashboard, fetchIncidentSummary } from './api';

const dashboard = ref<DashboardResponse>();
const selectedSender = ref<string>();
const incidentSummary = ref<IncidentSummaryResponse>();
const loading = ref(true);
const summaryLoading = ref(false);
const error = ref<string>();

const tagType: Record<OutpostSummary['riskLevel'], 'success' | 'warning' | 'error'> = {
  low: 'success',
  medium: 'warning',
  high: 'error',
  critical: 'error',
};

const columns: DataTableColumns<OutpostSummary> = [
  { title: 'Outpost', key: 'senderId' },
  { title: 'Location', key: 'location' },
  { title: 'Last seen', key: 'lastSeen' },
  { title: 'Hours silent', key: 'hoursSilent', sorter: (a, b) => a.hoursSilent - b.hoursSilent },
  { title: 'Reliability', key: 'reliabilityScore', render: (row) => `${row.reliabilityScore}%` },
  {
    title: 'Risk',
    key: 'riskLevel',
    render: (row) => h(NTag, { type: tagType[row.riskLevel], bordered: false }, { default: () => row.riskLevel }),
  },
];

const senderOptions = computed(() =>
  dashboard.value?.outposts.map((item) => ({ label: item.senderId, value: item.senderId })) ?? [],
);
const criticalCount = computed(
  () => dashboard.value?.outposts.filter((item) => item.riskLevel === 'critical').length ?? 0,
);
const unverifiedEmergencyCount = computed(
  () =>
    dashboard.value?.recentBroadcasts.filter(
      (item) => item.type === 'emergency' && item.crossCheckStatus !== 'verified',
    ).length ?? 0,
);

async function loadDashboard() {
  try {
    dashboard.value = await fetchDashboard();
    selectedSender.value = dashboard.value.outposts[0]?.senderId;
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Could not load dashboard.';
  } finally {
    loading.value = false;
  }
}

async function generateSummary() {
  if (!selectedSender.value) return;
  summaryLoading.value = true;
  error.value = undefined;
  try {
    incidentSummary.value = await fetchIncidentSummary(selectedSender.value);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : 'Could not generate summary.';
  } finally {
    summaryLoading.value = false;
  }
}

onMounted(loadDashboard);
</script>

<template>
  <NConfigProvider :theme="darkTheme">
    <NLayout class="app-shell">
      <NLayoutContent content-style="padding: 32px; max-width: 1180px; margin: 0 auto;">
        <header class="hero">
          <p class="eyebrow">Sunken Garden relay network</p>
          <h1>Silent Outposts</h1>
          <p>Find missing check-ins, inspect conflicting broadcasts, and send scouts before emergencies go unheard.</p>
        </header>

        <NAlert v-if="error" type="error" closable class="section" @close="error = undefined">
          {{ error }}
        </NAlert>

        <div v-if="loading" class="loading"><NSpin size="large" /></div>

        <template v-else-if="dashboard">
          <section class="stats-grid section">
            <NCard><NStatistic label="Tracked outposts" :value="dashboard.outposts.length" /></NCard>
            <NCard><NStatistic label="Critical silence alerts" :value="criticalCount" /></NCard>
            <NCard><NStatistic label="Unverified emergencies" :value="unverifiedEmergencyCount" /></NCard>
            <NCard><NStatistic label="Analysis time" :value="dashboard.analysisTimestamp" /></NCard>
          </section>

          <section class="content-grid section">
            <NCard title="Silence risk">
              <SilenceChart :outposts="dashboard.outposts" />
            </NCard>

            <NCard title="AI incident brief">
              <NSpace vertical :size="16">
                <NSelect v-model:value="selectedSender" :options="senderOptions" placeholder="Choose an outpost" />
                <NButton type="primary" :loading="summaryLoading" :disabled="!selectedSender" @click="generateSummary">
                  Analyse evidence
                </NButton>
                <NAlert v-if="incidentSummary" :type="incidentSummary.source === 'ai' ? 'info' : 'warning'">
                  <template #header>
                    {{ incidentSummary.source === 'ai' ? 'AI-generated brief' : 'Deterministic fallback' }}
                  </template>
                  {{ incidentSummary.summary }}
                  <p class="evidence">Evidence: {{ incidentSummary.evidenceIds.join(', ') || 'none' }}</p>
                </NAlert>
                <NEmpty v-else description="Select an outpost and analyse its evidence." />
              </NSpace>
            </NCard>
          </section>

          <NCard title="Outpost watchlist" class="section">
            <NDataTable :columns="columns" :data="dashboard.outposts" :row-key="(row: OutpostSummary) => row.senderId" />
          </NCard>
        </template>
      </NLayoutContent>
    </NLayout>
  </NConfigProvider>
</template>

