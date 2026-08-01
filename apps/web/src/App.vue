<script setup lang="ts">
import { LockClosedOutline, MenuOutline, RadioOutline } from '@vicons/ionicons5';
import { computed, onMounted, onUnmounted, ref, watchEffect } from 'vue';
import {
  NAlert,
  NButton,
  NConfigProvider,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NIcon,
  NSpin,
  NTag,
  darkTheme,
  type GlobalThemeOverrides,
} from 'naive-ui';
import type {
  AuthAccount,
  AuthLoginResponse,
  DashboardResponse,
  HealthResponse,
  StationAccountSummary,
  StationSummary,
  StationsResponse,
} from '@vts/common';
import {
  fetchAuthSession,
  fetchDashboard,
  fetchHealth,
  fetchStationAccounts,
  fetchStations,
  getAuthToken,
  logoutStation,
  setAuthToken,
} from './api';
import AppSidebar, { type AppView } from './components/AppSidebar.vue';
import DisplayModeControl from './components/DisplayModeControl.vue';
import { useSessionPreferences } from './composables/useSessionPreferences';
import HomeView from './views/HomeView.vue';
import LoginView from './views/LoginView.vue';
import MessagesView from './views/MessagesView.vue';
import SecureChannelView from './views/SecureChannelView.vue';
import SettingsView from './views/SettingsView.vue';
import StatsView from './views/StatsView.vue';
import { formatTimestamp, titleCase } from './utils/format';

const currentView = ref<AppView>('home');
const dashboard = ref<DashboardResponse>();
const stationsResponse = ref<StationsResponse>();
const health = ref<HealthResponse>();
const stationAccounts = ref<StationAccountSummary[]>([]);
const currentAccount = ref<AuthAccount>();
const stations = ref<StationSummary[]>([]);
const selectedStationId = ref<string>();
const authLoading = ref(true);
const authError = ref<string>();
const loading = ref(true);
const error = ref<string>();
const drawerOpen = ref(false);
const isMobile = ref(false);
const { displayMode, savedPhrase, savePhrase, clearPhrase } = useSessionPreferences();
const activeStation = computed(() => stations.value.find((station) => station.senderId === selectedStationId.value));

const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const carrier = displayMode.value === 'carrier';
  const primary = carrier ? '#e1a84b' : '#4bc58b';
  const primaryHover = carrier ? '#efbc69' : '#67d6a1';
  return {
    common: {
      primaryColor: primary,
      primaryColorHover: primaryHover,
      primaryColorPressed: carrier ? '#bd7b2f' : '#2f9f6c',
      primaryColorSuppl: primaryHover,
      infoColor: carrier ? '#d97f4b' : '#58a6c4',
      bodyColor: carrier ? '#141211' : '#101513',
      cardColor: carrier ? '#1b1816' : '#171d1a',
      modalColor: carrier ? '#1b1816' : '#171d1a',
      popoverColor: carrier ? '#211c19' : '#1d2521',
      borderColor: carrier ? '#3a2f28' : '#29352f',
      textColorBase: '#e9efec',
      borderRadius: '4px',
    },
    Card: { borderColor: carrier ? '#332a25' : '#27332e' },
    Button: { borderRadiusMedium: '4px', borderRadiusSmall: '4px', borderRadiusLarge: '4px' },
    Input: { borderRadius: '4px' },
    Select: { peers: { InternalSelection: { borderRadius: '4px' } } },
  };
});

watchEffect(() => {
  document.documentElement.dataset.displayMode = displayMode.value;
});

function updateViewport() {
  isMobile.value = window.matchMedia('(max-width: 860px)').matches;
  if (!isMobile.value) drawerOpen.value = false;
}

function navigate(view: AppView) {
  currentView.value = view;
  drawerOpen.value = false;
}

function selectStation(senderId: string) {
  selectedStationId.value = senderId;
  currentView.value = 'messages';
  drawerOpen.value = false;
}

function updateStation(updated: StationSummary) {
  stations.value = stations.value
    .map((station) => station.senderId === updated.senderId ? updated : station)
    .sort((a, b) => new Date(b.lastBroadcastAt.replace(' ', 'T')).valueOf() - new Date(a.lastBroadcastAt.replace(' ', 'T')).valueOf());
}

async function loadStationAccounts() {
  authError.value = undefined;
  try {
    stationAccounts.value = (await fetchStationAccounts()).accounts;
  } catch (caught) {
    authError.value = caught instanceof Error ? caught.message : 'Station accounts are unavailable.';
  }
}

function clearApplicationData() {
  dashboard.value = undefined;
  stationsResponse.value = undefined;
  health.value = undefined;
  stations.value = [];
  selectedStationId.value = undefined;
  error.value = undefined;
}

async function loadApplication() {
  if (!currentAccount.value) return;
  loading.value = true;
  error.value = undefined;
  const [dashboardResult, stationsResult, healthResult] = await Promise.allSettled([
    fetchDashboard(),
    fetchStations(),
    fetchHealth(),
  ]);

  if (dashboardResult.status === 'fulfilled') dashboard.value = dashboardResult.value;
  if (stationsResult.status === 'fulfilled') {
    stationsResponse.value = stationsResult.value;
    stations.value = stationsResult.value.stations;
    selectedStationId.value = stations.value.some((station) => station.senderId === currentAccount.value?.stationId)
      ? currentAccount.value?.stationId
      : stations.value[0]?.senderId;
  }
  if (healthResult.status === 'fulfilled') health.value = healthResult.value;

  const failures = [dashboardResult, stationsResult].filter((result) => result.status === 'rejected');
  if (failures.length) {
    error.value = failures.length === 2
      ? 'The server is unavailable. Start the data-only server and retry.'
      : 'Some network data could not be loaded. Available views remain accessible.';
  }
  loading.value = false;
}

async function initializeAuth() {
  authLoading.value = true;
  if (getAuthToken()) {
    try {
      currentAccount.value = (await fetchAuthSession()).account;
    } catch {
      setAuthToken(undefined);
      currentAccount.value = undefined;
    }
  }
  if (!currentAccount.value) {
    await loadStationAccounts();
  }
  authLoading.value = false;
  if (currentAccount.value) {
    await loadApplication();
  } else {
    loading.value = false;
  }
}

function handleSignedIn(response: AuthLoginResponse) {
  setAuthToken(response.token);
  currentAccount.value = response.account;
  authError.value = undefined;
  void loadApplication();
}

async function signOut() {
  try {
    await logoutStation();
  } catch {
    // Local session cleanup still matters if the server is already unreachable.
  }
  setAuthToken(undefined);
  currentAccount.value = undefined;
  currentView.value = 'home';
  drawerOpen.value = false;
  clearApplicationData();
  await loadStationAccounts();
  loading.value = false;
}

onMounted(() => {
  updateViewport();
  window.addEventListener('resize', updateViewport);
  void initializeAuth();
});
onUnmounted(() => window.removeEventListener('resize', updateViewport));
</script>

<template>
  <NConfigProvider :theme="darkTheme" :theme-overrides="themeOverrides">
    <div v-if="authLoading" class="auth-loading">
      <NSpin size="large" />
      <span>Checking station session…</span>
    </div>

    <LoginView
      v-else-if="!currentAccount"
      :accounts="stationAccounts"
      :loading="authLoading"
      :error="authError"
      @signed-in="handleSignedIn"
      @retry="loadStationAccounts"
    />

    <div v-else :class="['app-frame', `mode-${displayMode}`]">
      <AppSidebar
        v-if="!isMobile"
        :current-view="currentView"
        :stations="stations"
        :selected-station-id="selectedStationId"
        :analysis-timestamp="stationsResponse?.analysisTimestamp"
        :account="currentAccount"
        :loading="loading"
        @navigate="navigate"
        @select-station="selectStation"
        @sign-out="signOut"
      />

      <NDrawer v-model:show="drawerOpen" placement="left" :width="286">
        <NDrawerContent body-content-style="padding: 0;">
          <AppSidebar
            :current-view="currentView"
            :stations="stations"
            :selected-station-id="selectedStationId"
            :analysis-timestamp="stationsResponse?.analysisTimestamp"
            :account="currentAccount"
            :loading="loading"
            @navigate="navigate"
            @select-station="selectStation"
            @sign-out="signOut"
          />
        </NDrawerContent>
      </NDrawer>

      <div class="workspace">
        <header class="topbar">
          <div class="mobile-brand">
            <NButton quaternary circle title="Open navigation" @click="drawerOpen = true">
              <template #icon><NIcon :component="MenuOutline" /></template>
            </NButton>
            <NIcon :component="RadioOutline" class="mobile-brand-icon" />
            <strong>{{ currentView === 'messages' && activeStation ? activeStation.senderId : 'Silent Outposts' }}</strong>
          </div>
          <div v-if="currentView === 'messages' && activeStation && !isMobile" class="topbar-station">
            <span class="topbar-station-icon"><NIcon :component="RadioOutline" /></span>
            <div class="topbar-station-copy">
              <strong>{{ activeStation.senderId }}</strong>
              <span>{{ activeStation.location }} · {{ titleCase(activeStation.senderType) }}</span>
            </div>
            <div class="topbar-station-facts">
              <NTag
                size="small"
                :type="activeStation.status === 'active' ? 'success' : 'warning'"
                :bordered="false"
              >
                {{ titleCase(activeStation.status) }}
              </NTag>
              <span>{{ activeStation.broadcastCount }} broadcasts</span>
              <span class="topbar-last-signal">Last signal {{ formatTimestamp(activeStation.lastBroadcastAt) }}</span>
            </div>
          </div>
          <div v-else class="view-context">
            <span>Sunken Garden relay network</span>
            <strong>{{ currentView }}</strong>
          </div>
          <div class="topbar-actions">
            <span v-if="currentView !== 'messages'" class="account-chip" :title="`Signed in as ${currentAccount.stationId}`">
              <NIcon :component="RadioOutline" />
              <span>{{ currentAccount.stationId }}</span>
            </span>
            <DisplayModeControl v-model="displayMode" />
            <NButton quaternary circle title="Sign out" @click="signOut">
              <template #icon><NIcon :component="LockClosedOutline" /></template>
            </NButton>
          </div>
        </header>

        <main :class="['main-content', { 'messages-content': currentView === 'messages' }]">
          <NAlert v-if="error" type="error" closable class="global-alert" @close="error = undefined">
            {{ error }}
          </NAlert>
          <NButton v-if="error" size="small" class="global-retry" @click="loadApplication">Retry connection</NButton>

          <div v-if="loading" class="loading-state">
            <NSpin size="large" />
            <span>Loading broadcast network…</span>
          </div>

          <template v-else>
            <HomeView
              v-if="currentView === 'home' && dashboard"
              :dashboard="dashboard"
              :stations="stations"
              :health="health"
              :display-mode="displayMode"
              @open-station="selectStation"
            />
            <StatsView v-if="currentView === 'stats' && dashboard" :dashboard="dashboard" />
            <SecureChannelView
              v-if="currentView === 'secure'"
              :stations="stations"
              :health="health"
              :current-station-id="currentAccount.stationId"
              :secret-phrase="savedPhrase"
              @open-settings="navigate('settings')"
            />
            <MessagesView
              v-if="stations.length"
              v-show="currentView === 'messages'"
              :stations="stations"
              :selected-station-id="selectedStationId"
              :analysis-timestamp="stationsResponse?.analysisTimestamp"
              :display-mode="displayMode"
              :secret-phrase="savedPhrase"
              :current-station-id="currentAccount.stationId"
              @select-station="selectStation"
              @station-updated="updateStation"
              @open-settings="navigate('settings')"
            />
            <SettingsView
              v-if="currentView === 'settings'"
              :saved-phrase="savedPhrase"
              :health="health"
              :account="currentAccount"
              @save-phrase="savePhrase"
              @clear-phrase="clearPhrase"
            />
            <NEmpty
              v-if="(currentView === 'home' || currentView === 'stats') && !dashboard || currentView === 'messages' && !stations.length"
              description="This view needs the server dataset to be available."
            >
              <template #extra><NButton @click="loadApplication">Retry connection</NButton></template>
            </NEmpty>
          </template>
        </main>
      </div>
    </div>
  </NConfigProvider>
</template>
