<script setup lang="ts">
import { computed } from 'vue';
import { BarChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { use } from 'echarts/core';
import VChart from 'vue-echarts';
import type { OutpostSummary } from '@vts/common';

use([BarChart, GridComponent, TooltipComponent, CanvasRenderer]);

const props = defineProps<{ outposts: OutpostSummary[] }>();

const colors: Record<OutpostSummary['riskLevel'], string> = {
  low: '#53c28b',
  medium: '#e5b567',
  high: '#e07a5f',
  critical: '#ef476f',
};

const option = computed(() => ({
  backgroundColor: 'transparent',
  grid: { top: 12, right: 20, bottom: 32, left: 110 },
  tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
  xAxis: {
    type: 'value',
    name: 'hours silent',
    nameTextStyle: { color: '#95a39e' },
    axisLabel: { color: '#95a39e' },
    splitLine: { lineStyle: { color: '#26332f' } },
  },
  yAxis: {
    type: 'category',
    data: props.outposts.map((item) => item.senderId).reverse(),
    axisLabel: { color: '#dfe8e4' },
    axisLine: { show: false },
    axisTick: { show: false },
  },
  series: [
    {
      type: 'bar',
      barWidth: 18,
      data: props.outposts
        .map((item) => ({ value: item.hoursSilent, itemStyle: { color: colors[item.riskLevel], borderRadius: 4 } }))
        .reverse(),
    },
  ],
}));
</script>

<template>
  <VChart class="chart" :option="option" autoresize />
</template>

<style scoped>
.chart {
  width: 100%;
  height: 280px;
}
</style>
