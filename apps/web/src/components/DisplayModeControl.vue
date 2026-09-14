<script setup lang="ts">
import { LockClosedOutline, LockOpenOutline } from '@vicons/ionicons5';
import { NIcon, NRadioButton, NRadioGroup } from 'naive-ui';
import type { DisplayMode } from '../composables/useSessionPreferences';

defineProps<{ modelValue: DisplayMode }>();
defineEmits<{ 'update:modelValue': [value: DisplayMode] }>();
</script>

<template>
  <NRadioGroup
    :value="modelValue"
    size="small"
    aria-label="Message display mode"
    @update:value="$emit('update:modelValue', $event)"
  >
    <NRadioButton value="decoded">
      <span class="mode-option"><NIcon :component="LockOpenOutline" />Decoded</span>
    </NRadioButton>
    <NRadioButton value="carrier">
      <span class="mode-option"><NIcon :component="LockClosedOutline" />Carrier</span>
    </NRadioButton>
  </NRadioGroup>
</template>

<style scoped>
.mode-option { display: inline-flex; align-items: center; gap: 6px; }
</style>
