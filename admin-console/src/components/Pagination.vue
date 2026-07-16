<script setup>
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

defineProps({
  data: { type: Object, required: true },
  pageCount: { type: Number, required: true },
})

defineEmits(['change'])
</script>

<template>
  <div class="pagination">
    <button class="icon-button bordered" type="button" title="上一页" :disabled="data.page <= 1" @click="$emit('change', data.page - 1)">
      <ChevronLeft :size="17" />
    </button>
    <span>第 {{ data.page }} / {{ pageCount }} 页</span>
    <button class="icon-button bordered" type="button" title="下一页" :disabled="data.page >= pageCount" @click="$emit('change', data.page + 1)">
      <ChevronRight :size="17" />
    </button>
  </div>
</template>
