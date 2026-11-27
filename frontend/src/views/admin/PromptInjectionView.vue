<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-[1440px]">
      <InstructionLayersPanel />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import InstructionLayersPanel from '@/components/admin/settings/InstructionLayersPanel.vue'
</script>
