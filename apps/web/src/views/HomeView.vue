<script setup lang="ts">
import { AlertCircleOutline, PulseOutline, RadioOutline, ShieldCheckmarkOutline } from '@vicons/ionicons5';
import { computed } from 'vue';
import { NButton, NCard, NEmpty, NIcon, NStatistic, NTag } from 'naive-ui';
import type { DashboardResponse, HealthResponse, StationSummary } from '@vts/common';
import BroadcastItem from '../components/BroadcastItem.vue';
import type { DisplayMode } from '../composables/useSessionPreferences';

const props = defineProps<{
  dashboard: DashboardResponse;
  stations: StationSummary[];
  health?: HealthResponse;
  displayMode: DisplayMode;
}>();

const emit = defineEmits<{ openStation: [senderId: string] }>();
const criticalCount = computed(() => props.dashboard.outposts.filter((item) => item.riskLevel === 'critical').length);
const encryptedCount = computed(() => props.dashboard.recentBroadcasts.filter((item) => item.encryptionStatus === 'encrypted').length);
const elevatedOutposts = computed(() => props.dashboard.outposts
  .filter((item) => item.riskLevel === 'critical' || item.riskLevel === 'high')
  .slice(0, 4));
</script>

<template>
  <div class="view-stack">
    <header class="view-header">
      <div>
        <span class="eyebrow">Network overview</span>
        <h1>Good morning, operator</h1>
        <p>Monitor the relay network and follow transmissions that need attention.</p>
      </div>
      <NTag :type="health?.ok ? 'success' : 'warning'" :bordered="false">
        <template #icon><NIcon :component="PulseOutline" /></template>
        {{ health?.ok ? 'Network online' : 'Status unavailable' }}
      </NTag>
    </header>

    <section class="metric-grid" aria-label="Network metrics">
      <NCard>
        <div class="metric-icon"><NIcon :component="RadioOutline" /></div>
        <NStatistic label="Stations" :value="health?.data.stationCount ?? stations.length" />
        <span class="metric-note">Across the Sunken Garden network</span>
      </NCard>
      <NCard>
        <div class="metric-icon alert"><NIcon :component="AlertCircleOutline" /></div>
        <NStatistic label="Critical silence" :value="criticalCount" />
        <span class="metric-note">Outposts requiring investigation</span>
      </NCard>
      <NCard>
        <div class="metric-icon"><NIcon :component="ShieldCheckmarkOutline" /></div>
        <NStatistic label="Secure broadcasts" :value="encryptedCount" />
        <span class="metric-note">In the current activity window</span>
      </NCard>
    </section>

    <NCard title="Outposts needing attention" class="risk-card">
      <NEmpty v-if="!elevatedOutposts.length" description="No high-risk outposts at this analysis time." />
      <div v-else class="risk-list">
        <button v-for="outpost in elevatedOutposts" :key="outpost.senderId" type="button" @click="emit('openStation', outpost.senderId)">
          <div><strong>{{ outpost.senderId }}</strong><span>{{ outpost.location }}</span></div>
          <div class="risk-copy">
            <NTag :type="outpost.riskLevel === 'critical' ? 'error' : 'warning'" size="small" :bordered="false">{{ outpost.riskLevel }}</NTag>
            <span>{{ outpost.hoursSilent.toFixed(1) }}h silent · {{ (outpost.silenceRatio ?? outpost.hoursSilent / 24).toFixed(1) }}× cadence</span>
            <span>{{ outpost.riskReasons?.[0]?.message || 'Silence exceeds the expected check-in window.' }}</span>
          </div>
        </button>
      </div>
    </NCard>

    <NCard title="Recent network traffic" class="feed-card">
      <template #header-extra>
        <span class="section-note">{{ dashboard.recentBroadcasts.length }} latest broadcasts</span>
      </template>
      <NEmpty v-if="!dashboard.recentBroadcasts.length" description="No broadcasts have been received." />
      <div v-else>
        <div v-for="broadcast in dashboard.recentBroadcasts.slice(0, 8)" :key="broadcast.id" class="home-broadcast">
          <BroadcastItem :broadcast="broadcast" :display-mode="displayMode" />
          <NButton text size="small" @click="emit('openStation', broadcast.senderId)">Open station</NButton>
        </div>
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.metric-icon { position: absolute; top: 18px; right: 18px; display: grid; width: 34px; height: 34px; color: var(--accent); background: var(--accent-soft); place-items: center; }
.metric-icon.alert { color: var(--danger); background: var(--danger-soft); }
.metric-note { display: block; margin-top: 8px; color: var(--text-faint); font-size: 0.72rem; }
.feed-card { min-height: 320px; }
.risk-list > button { display: grid; width: 100%; padding: 13px 0; border: 0; border-bottom: 1px solid var(--border); color: var(--text); background: transparent; cursor: pointer; grid-template-columns: minmax(150px, 0.35fr) minmax(0, 1fr); gap: 18px; text-align: left; }
.risk-list > button:hover { background: var(--surface-hover); }
.risk-list > button:last-child { border-bottom: 0; }
.risk-list strong, .risk-list span { display: block; }
.risk-list strong { color: var(--text-strong); }
.risk-list span { margin-top: 3px; color: var(--text-faint); font-size: 0.72rem; }
.risk-copy { display: grid; grid-template-columns: auto minmax(150px, 0.35fr) minmax(220px, 1fr); align-items: center; gap: 12px; }
.risk-copy span { margin: 0; }
.section-note { color: var(--text-faint); font-size: 0.72rem; }
.home-broadcast { position: relative; }
.home-broadcast > :last-child { position: absolute; right: 0; bottom: 18px; }
@media (max-width: 640px) { .home-broadcast > :last-child { position: static; margin: -10px 0 15px 24px; } }
@media (max-width: 720px) { .risk-list > button, .risk-copy { grid-template-columns: 1fr; gap: 6px; } }
</style>
