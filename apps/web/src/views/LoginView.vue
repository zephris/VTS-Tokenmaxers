<script setup lang="ts">
import { KeyOutline, RadioOutline, RefreshOutline } from '@vicons/ionicons5';
import { computed, ref, watch } from 'vue';
import { NAlert, NButton, NCard, NFormItem, NIcon, NInput, NSelect, NSpin, NTag } from 'naive-ui';
import type { AuthLoginResponse, StationAccountSummary } from '@vts/common';
import { loginStation } from '../api';
import { titleCase } from '../utils/format';

const props = defineProps<{
  accounts: StationAccountSummary[];
  loading?: boolean;
  error?: string;
}>();

const emit = defineEmits<{
  signedIn: [response: AuthLoginResponse];
  retry: [];
}>();

const username = ref('');
const password = ref('');
const signingIn = ref(false);
const loginError = ref<string>();

const accountOptions = computed(() => props.accounts.map((account) => ({
  label: `${account.stationId} · ${account.location || titleCase(account.senderType)}`,
  value: account.username,
})));
const selectedAccount = computed(() => props.accounts.find((account) => account.username === username.value));
const canSubmit = computed(() => username.value.length > 0 && password.value.length > 0 && !signingIn.value);

watch(() => props.accounts, (accounts) => {
  if (!username.value && accounts.length) username.value = accounts[0]?.username ?? '';
}, { immediate: true });

async function submit() {
  if (!canSubmit.value) return;
  signingIn.value = true;
  loginError.value = undefined;
  try {
    const response = await loginStation({ username: username.value, password: password.value });
    password.value = '';
    emit('signedIn', response);
  } catch (caught) {
    loginError.value = caught instanceof Error ? caught.message : 'Station sign-in failed.';
  } finally {
    signingIn.value = false;
  }
}
</script>

<template>
  <div class="login-view">
    <NCard class="login-card">
      <template #header>
        <div class="login-title">
          <span class="login-mark"><NIcon :component="RadioOutline" size="22" /></span>
          <div>
            <strong>Silent Outposts</strong>
            <span>Station identity required</span>
          </div>
        </div>
      </template>

      <NSpin :show="loading">
        <div class="login-body">
          <NAlert v-if="error" type="error" closable>
            <div class="login-alert-content">
              <span>{{ error }}</span>
              <NButton size="small" @click="emit('retry')">
                <template #icon><NIcon :component="RefreshOutline" /></template>
                Retry
              </NButton>
            </div>
          </NAlert>
          <NAlert v-if="loginError" type="error" closable @close="loginError = undefined">
            {{ loginError }}
          </NAlert>

          <NFormItem label="Station account">
            <NSelect
              v-model:value="username"
              :options="accountOptions"
              filterable
              placeholder="Choose your station"
              :disabled="loading || signingIn || !accounts.length"
            />
          </NFormItem>

          <div v-if="selectedAccount" class="selected-account">
            <span>{{ selectedAccount.location || 'No current location' }}</span>
            <NTag size="small" :bordered="false">{{ titleCase(selectedAccount.senderType) }}</NTag>
          </div>

          <NFormItem label="Password">
            <NInput
              v-model:value="password"
              type="password"
              show-password-on="click"
              placeholder="Default local password: station-demo"
              maxlength="160"
              autocomplete="current-password"
              :disabled="loading || signingIn"
              @keyup.enter="submit"
            >
              <template #prefix><NIcon :component="KeyOutline" /></template>
            </NInput>
          </NFormItem>

          <NButton type="primary" block size="large" :loading="signingIn" :disabled="!canSubmit" @click="submit">
            <template #icon><NIcon :component="KeyOutline" /></template>
            Sign in
          </NButton>
        </div>
      </NSpin>
    </NCard>
  </div>
</template>

<style scoped>
.login-view {
  display: grid;
  min-height: 100vh;
  padding: 24px;
  background:
    linear-gradient(180deg, rgba(75, 197, 139, 0.07), transparent 38%),
    var(--page);
  place-items: center;
}

.login-card {
  width: min(100%, 430px);
  border-color: var(--border-strong);
  background: var(--surface);
}

.login-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.login-title strong,
.login-title span {
  display: block;
}

.login-title strong {
  color: var(--text-strong);
  font-size: 1.05rem;
}

.login-title span {
  margin-top: 2px;
  color: var(--text-faint);
  font-size: 0.72rem;
  text-transform: uppercase;
}

.login-mark {
  display: grid;
  width: 40px;
  height: 40px;
  border: 1px solid var(--accent);
  color: var(--accent);
  background: var(--accent-soft);
  place-items: center;
}

.login-body {
  display: grid;
  gap: 14px;
}

.login-alert-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.selected-account {
  display: flex;
  min-height: 34px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  color: var(--text-muted);
  background: var(--surface-raised);
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 0.76rem;
}

@media (max-width: 480px) {
  .login-view { padding: 14px; }
  .login-alert-content { align-items: flex-start; flex-direction: column; }
}
</style>
