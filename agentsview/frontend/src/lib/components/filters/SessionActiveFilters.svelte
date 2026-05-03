<script lang="ts">
  import { sessions } from "../../stores/sessions.svelte.js";
  import {
    agentColor,
    agentLabel,
  } from "../../utils/agents.js";
  import { t } from "../../i18n/index.js";

  interface Props {
    projectFilters?: string[];
    modelFilters?: string[];
    onRemoveProject?: (project: string) => void;
    onClearProjects?: () => void;
    onRemoveModel?: (model: string) => void;
    onClearModels?: () => void;
  }

  let {
    projectFilters = [],
    modelFilters = [],
    onRemoveProject,
    onClearProjects,
    onRemoveModel,
    onClearModels,
  }: Props = $props();

  const selectedAgents = $derived(
    sessions.filters.agent
      ? sessions.filters.agent.split(",")
      : [],
  );
  const selectedMachines = $derived(
    sessions.filters.machine
      ? sessions.filters.machine.split(",")
      : [],
  );

  const hasFilters = $derived(
    !!sessions.filters.project ||
      sessions.hasActiveFilters ||
      projectFilters.length > 0 ||
      modelFilters.length > 0,
  );

  function clearProject() {
    sessions.filters.project = "";
    sessions.activeSessionId = null;
    sessions.load();
  }

  function removeMachine(machine: string) {
    sessions.toggleMachineFilter(machine);
  }

  function removeAgent(agent: string) {
    sessions.toggleAgentFilter(agent);
  }

  function clearAll() {
    sessions.filters.project = "";
    sessions.clearSessionFilters();
    onClearProjects?.();
    onClearModels?.();
  }
</script>

{#if hasFilters}
  <div class="active-filters">
    <span class="filters-label">{t("filter.filters")}:</span>

    {#if sessions.filters.project}
      <button
        class="filter-chip"
        onclick={clearProject}
        title={t("filter.clear_project")}
      >
        {sessions.filters.project}
        <span class="chip-x">&times;</span>
      </button>
    {/if}

    {#each selectedMachines as machine (machine)}
      <button
        class="filter-chip"
        onclick={() => removeMachine(machine)}
        title={t("filter.remove_machine", { machine })}
      >
        {machine}
        <span class="chip-x">&times;</span>
      </button>
    {/each}

    {#each selectedAgents as agent (agent)}
      <button
        class="filter-chip"
        onclick={() => removeAgent(agent)}
        title={t("filter.remove_agent", { agent: agentLabel(agent) })}
      >
        <span
          class="agent-chip-dot"
          style:background={agentColor(agent)}
        ></span>
        {agentLabel(agent)}
        <span class="chip-x">&times;</span>
      </button>
    {/each}

    {#if sessions.filters.minUserMessages > 0}
      <button
        class="filter-chip"
        onclick={() => sessions.setMinUserMessagesFilter(0)}
        title={t("filter.clear_min_prompts")}
      >
        {t("filter.label_min_prompts", { n: sessions.filters.minUserMessages })}
        <span class="chip-x">&times;</span>
      </button>
    {/if}

    {#if sessions.filters.recentlyActive}
      <button
        class="filter-chip"
        onclick={() => sessions.setRecentlyActiveFilter(false)}
        title={t("filter.clear_recently")}
      >
        {t("filter.label_active_24h")}
        <span class="chip-x">&times;</span>
      </button>
    {/if}

    {#if sessions.filters.hideUnknownProject}
      <button
        class="filter-chip"
        onclick={() => sessions.setHideUnknownProjectFilter(false)}
        title={t("filter.clear_hidden_unknown")}
      >
        {t("filter.label_unknown_hidden")}
        <span class="chip-x">&times;</span>
      </button>
    {/if}

    {#each projectFilters as project (project)}
      <button
        class="filter-chip"
        onclick={() => onRemoveProject?.(project)}
        title={t("filter.remove_project", { project })}
      >
        {project}
        <span class="chip-x">&times;</span>
      </button>
    {/each}

    {#if !sessions.filters.includeOneShot}
      <button
        class="filter-chip"
        onclick={() => sessions.setIncludeOneShotFilter(true)}
        title={t("filter.clear_single_turn")}
      >
        {t("filter.label_single_turn_hidden")}
        <span class="chip-x">&times;</span>
      </button>
    {/if}

    {#if sessions.filters.includeAutomated}
      <button
        class="filter-chip"
        onclick={() => sessions.setIncludeAutomatedFilter(false)}
        title={t("filter.clear_automated")}
      >
        {t("filter.label_automated_included")}
        <span class="chip-x">&times;</span>
      </button>
    {/if}

    {#each modelFilters as model (model)}
      <button
        class="filter-chip"
        onclick={() => onRemoveModel?.(model)}
        title={t("filter.remove_model", { model })}
      >
        {model}
        <span class="chip-x">&times;</span>
      </button>
    {/each}

    <button
      class="clear-all"
      onclick={clearAll}
      title={t("filter.clear_all")}
    >
      {t("filter.clear_all")}
    </button>
  </div>
{/if}

<style>
  .active-filters {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 16px 6px;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-muted);
    flex-shrink: 0;
    flex-wrap: wrap;
  }

  .filters-label {
    font-size: 10px;
    font-weight: 500;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }

  .filter-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 22px;
    padding: 0 6px;
    font-size: 11px;
    font-weight: 500;
    color: var(--accent-blue);
    background: color-mix(
      in srgb, var(--accent-blue) 10%, transparent
    );
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background 0.1s;
  }

  .filter-chip:hover {
    background: color-mix(
      in srgb, var(--accent-blue) 18%, transparent
    );
  }

  .agent-chip-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .chip-x {
    opacity: 0.65;
    font-size: 12px;
    line-height: 1;
  }

  .clear-all {
    font-size: 11px;
    color: var(--text-muted);
    padding: 2px 6px;
    border-radius: var(--radius-sm);
    cursor: pointer;
  }

  .clear-all:hover {
    color: var(--text-primary);
    background: var(--bg-surface-hover);
  }
</style>
