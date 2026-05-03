<script lang="ts">
  import { t } from "../../i18n/index.js";
  import { getOutcomeColor } from "../../utils/grade.js";

  interface Props {
    distribution: Record<string, number>;
  }

  let { distribution }: Props = $props();

  const outcomeKeys: Record<string, string> = {
    completed: "analytics.outcome_completed",
    abandoned: "analytics.outcome_abandoned",
    errored: "analytics.outcome_errored",
    unknown: "analytics.outcome_unknown",
  };

  const outcomes = [
    "completed", "abandoned", "errored", "unknown",
  ] as const;

  const total = $derived(
    outcomes.reduce(
      (sum, o) => sum + (distribution[o] ?? 0), 0,
    ),
  );
</script>

<div class="outcome-dist">
  <div class="chart-title">{t("analytics.outcome_distribution")}</div>
  {#if total > 0}
    <div class="stacked-bar">
      {#each outcomes as outcome}
        {@const count = distribution[outcome] ?? 0}
        {#if count > 0}
          <div
            class="segment"
            style:width="{(count / total) * 100}%"
            style:background={getOutcomeColor(outcome)}
            title="{t(outcomeKeys[outcome]!)}: {count}"
          ></div>
        {/if}
      {/each}
    </div>
    <div class="legend">
      {#each outcomes as outcome}
        {@const count = distribution[outcome] ?? 0}
        {#if count > 0}
          <div class="legend-item">
            <span
              class="legend-dot"
              style:background={getOutcomeColor(outcome)}
            ></span>
            <span class="legend-text">
              {t(outcomeKeys[outcome]!)} {count}
            </span>
          </div>
        {/if}
      {/each}
    </div>
  {:else}
    <div class="empty">{t("analytics.no_outcome_data")}</div>
  {/if}
</div>

<style>
  .chart-title {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 10px;
  }
  .stacked-bar {
    display: flex;
    height: 24px;
    border-radius: 4px;
    overflow: hidden;
    margin-bottom: 10px;
  }
  .segment {
    transition: width 0.3s ease;
  }
  .legend {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
  }
  .legend-item {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
  }
  .legend-dot {
    width: 8px;
    height: 8px;
    border-radius: 2px;
  }
  .legend-text {
    color: var(--text-secondary);
  }
  .empty {
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
