<script setup lang="ts">
import { computed, ref, type Component } from 'vue';
import {
  BarChartOutline,
  ChatbubblesOutline,
  ChevronDownOutline,
  HardwareChipOutline,
  HomeOutline,
  LockClosedOutline,
  PeopleOutline,
  RadioOutline,
  SettingsOutline,
} from '@vicons/ionicons5';
import { NButton, NCollapseTransition, NIcon, NScrollbar, NSpin } from 'naive-ui';
import type { AuthAccount, StationSummary } from '@vts/common';
import { formatRelativeTime } from '../utils/format';

export type AppView = 'home' | 'secure' | 'stats' | 'messages' | 'settings';

const props = defineProps<{
  currentView: AppView;
  stations: StationSummary[];
  selectedStationId?: string;
  analysisTimestamp?: string;
  account?: AuthAccount;
  loading?: boolean;
}>();

const emit = defineEmits<{
  navigate: [view: AppView];
  selectStation: [senderId: string];
  signOut: [];
}>();

const messagesExpanded = ref(true);
const navigation: Array<{ label: string; value: AppView; icon: Component }> = [
  { label: 'Home', value: 'home', icon: HomeOutline },
  { label: 'Secure channel', value: 'secure', icon: LockClosedOutline },
  { label: 'Stats', value: 'stats', icon: BarChartOutline },
];

function stationIcon(type: StationSummary['senderType']): Component {
  if (type === 'robot_outpost') return HardwareChipOutline;
  if (type === 'junior_scout_group') return PeopleOutline;
  return RadioOutline;
}

const stationCountLabel = computed(() => (props.loading ? '' : String(props.stations.length)));

function openMessages() {
  messagesExpanded.value = true;
  emit('navigate', 'messages');
}

function chooseStation(senderId: string) {
  emit('selectStation', senderId);
  emit('navigate', 'messages');
}
</script>

<template>
  <aside class="sidebar" aria-label="Primary navigation">
    <header class="brand-block">
      <div class="brand-mark" aria-hidden="true"><span /></div>
      <div>
        <strong>Silent Outposts</strong>
        <span>Broadcast network</span>
      </div>
    </header>

    <nav class="nav-list">
      <NButton
        v-for="item in navigation"
        :key="item.value"
        quaternary
        block
        :class="['nav-button', { active: currentView === item.value }]"
        @click="emit('navigate', item.value)"
      >
        <template #icon><NIcon :component="item.icon" /></template>
        {{ item.label }}
      </NButton>

      <div class="messages-nav">
        <NButton
          quaternary
          block
          :class="['nav-button', { active: currentView === 'messages' }]"
          @click="openMessages"
        >
          <template #icon><NIcon :component="ChatbubblesOutline" /></template>
          <span class="messages-label">Station traffic</span>
          <span class="messages-suffix">
            <span class="nav-count">{{ stationCountLabel }}</span>
            <NIcon
              :component="ChevronDownOutline"
              :class="['expand-icon', { expanded: messagesExpanded }]"
              @click.stop="messagesExpanded = !messagesExpanded"
            />
          </span>
        </NButton>

        <NCollapseTransition :show="messagesExpanded">
          <div class="station-scroll">
            <NSpin v-if="loading" size="small" class="station-loading" />
            <NScrollbar v-else style="max-height: calc(100vh - 390px)">
              <button
                v-for="station in stations"
                :key="station.senderId"
                type="button"
                :class="['station-link', { active: selectedStationId === station.senderId && currentView === 'messages' }]"
                @click="chooseStation(station.senderId)"
              >
                <NIcon :component="stationIcon(station.senderType)" size="18" />
                <span class="station-copy">
                  <span class="station-line">
                    <strong>{{ station.senderId }}</strong>
                    <time>{{ formatRelativeTime(station.lastBroadcastAt, analysisTimestamp) }}</time>
                  </span>
                  <span>{{ station.lastMessagePreview || 'No readable transmission' }}</span>
                </span>
              </button>
            </NScrollbar>
          </div>
        </NCollapseTransition>
      </div>

      <NButton
        quaternary
        block
        :class="['nav-button', { active: currentView === 'settings' }]"
        @click="emit('navigate', 'settings')"
      >
        <template #icon><NIcon :component="SettingsOutline" /></template>
        Settings
      </NButton>
    </nav>

    <footer class="sidebar-footer">
      <div class="identity-line">
        <span class="status-dot" />
        <span>{{ account?.stationId ?? 'Station account' }}</span>
      </div>
      <NButton size="tiny" secondary @click="emit('signOut')">Sign out</NButton>
    </footer>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  width: 286px;
  height: 100%;
  padding: 20px 14px 14px;
  border-right: 1px solid var(--border);
  background: var(--sidebar);
  flex-direction: column;
}

.brand-block {
  display: flex;
  min-height: 58px;
  padding: 2px 10px 18px;
  align-items: center;
  gap: 12px;
}

.brand-block strong,
.brand-block span {
  display: block;
}

.brand-block strong {
  color: var(--text-strong);
  font-size: 1rem;
}

.brand-block span {
  margin-top: 2px;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-transform: uppercase;
}

.brand-mark {
  display: grid;
  width: 34px;
  height: 34px;
  border: 1px solid var(--accent);
  place-items: center;
}

.brand-mark::before,
.brand-mark::after,
.brand-mark span {
  display: block;
  width: 3px;
  background: var(--accent);
  content: '';
}

.brand-mark::before { height: 10px; }
.brand-mark span { height: 18px; }
.brand-mark::after { height: 25px; }
.brand-mark { grid-template-columns: repeat(3, 3px); gap: 4px; align-items: end; }

.nav-list {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  gap: 4px;
}

.nav-button {
  justify-content: flex-start;
  height: 42px;
  padding: 0 12px;
  color: var(--text-muted);
  font-weight: 600;
}

.nav-button.active {
  color: var(--accent);
  background: var(--accent-soft);
}

.messages-nav { min-height: 0; }
.nav-count { color: var(--text-faint); font-size: 0.72rem; }
.messages-label { flex: 1; text-align: left; }
.messages-suffix { display: inline-flex; align-items: center; gap: 6px; }
.expand-icon { transition: transform 160ms ease; }
.expand-icon.expanded { transform: rotate(180deg); }

.station-scroll {
  margin: 4px 0 6px 18px;
  border-left: 1px solid var(--border);
}

.station-loading { display: block; margin: 18px auto; }

.station-link {
  display: grid;
  width: 100%;
  min-height: 58px;
  padding: 8px 8px 8px 12px;
  border: 0;
  color: var(--text-muted);
  background: transparent;
  cursor: pointer;
  grid-template-columns: 20px minmax(0, 1fr);
  gap: 8px;
  text-align: left;
}

.station-link:hover,
.station-link.active { color: var(--text-strong); background: var(--surface-hover); }
.station-link.active { box-shadow: inset 2px 0 var(--accent); }
.station-copy { min-width: 0; }
.station-copy > span:last-child { display: block; overflow: hidden; margin-top: 3px; color: var(--text-faint); font-size: 0.72rem; text-overflow: ellipsis; white-space: nowrap; }
.station-line { display: flex; min-width: 0; justify-content: space-between; gap: 6px; }
.station-line strong { overflow: hidden; font-size: 0.78rem; text-overflow: ellipsis; white-space: nowrap; }
.station-line time { color: var(--text-faint); font-size: 0.66rem; white-space: nowrap; }

.sidebar-footer {
  display: flex;
  padding: 14px 10px 2px;
  border-top: 1px solid var(--border);
  color: var(--text-faint);
  font-size: 0.72rem;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.identity-line {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 7px;
}

.identity-line span:last-child {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 8px var(--accent);
}
</style>
