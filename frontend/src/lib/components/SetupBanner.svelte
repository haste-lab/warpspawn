<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();
</script>

<div class="banner">
  <div class="banner-content">
    <span class="banner-icon">🔧</span>
    <div>
      <strong>Setup required</strong>
      <span class="text-muted"> — Configure an LLM provider to start building projects autonomously.</span>
    </div>
  </div>
  <button class="btn btn-primary btn-sm" on:click={() => dispatch('setup')}>
    Complete Setup
  </button>
</div>

<style>
  .banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 24px;
    background: var(--amber-dim);
    border-bottom: 1px solid rgba(255, 184, 108, 0.15);
  }
  .banner-content {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 0.85rem;
  }
  .banner-icon { font-size: 1rem; }
</style>
