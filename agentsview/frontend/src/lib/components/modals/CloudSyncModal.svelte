<script lang="ts">
  import {
    getCloudSyncStatus,
    uploadCloudSync,
    downloadCloudSync,
    type CloudSyncStatus,
    type CloudSyncStats,
    type CloudSyncEvent,
  } from "../../api/client.js";
  import { t } from "../../i18n/index.js";

  interface Props {
    open: boolean;
    onclose: () => void;
    onsynced: () => void;
  }

  let {
    open = $bindable(),
    onclose,
    onsynced,
  }: Props = $props();

  type View = "status" | "running" | "done" | "error";

  let view: View = $state("status");
  let errorMessage: string = $state("");
  let status: CloudSyncStatus | null = $state(null);
  let stats: CloudSyncStats | null = $state(null);
  let operation: string = $state("");
  let abortFn: (() => void) | null = $state(null);

  async function loadStatus() {
    try {
      status = await getCloudSyncStatus();
    } catch (err) {
      // status unavailable — show empty
    }
  }

  function handleEvent(ev: CloudSyncEvent) {
    switch (ev.type) {
      case "started":
        operation = ev.operation;
        break;
      case "done":
        stats = ev.stats;
        view = "done";
        loadStatus();
        onsynced();
        break;
      case "error":
        errorMessage = ev.message;
        view = "error";
        break;
    }
  }

  async function handleUpload() {
    view = "running";
    errorMessage = "";
    stats = null;
    const stream = uploadCloudSync(handleEvent);
    abortFn = () => stream.abort();
    try {
      await stream.done;
    } catch (err) {
      if (view === "running") {
        errorMessage = err instanceof Error ? err.message : t("modal.cloud_sync.failed");
        view = "error";
      }
    }
    abortFn = null;
  }

  async function handleDownload() {
    view = "running";
    errorMessage = "";
    stats = null;
    const stream = downloadCloudSync(handleEvent);
    abortFn = () => stream.abort();
    try {
      await stream.done;
    } catch (err) {
      if (view === "running") {
        errorMessage = err instanceof Error ? err.message : t("modal.cloud_sync.failed");
        view = "error";
      }
    }
    abortFn = null;
  }

  function handleClose() {
    if (view === "running") return;
    view = "status";
    errorMessage = "";
    stats = null;
    open = false;
    onclose();
  }

  // Load status when modal opens.
  $effect(() => {
    if (open) {
      view = "status";
      errorMessage = "";
      stats = null;
      loadStatus();
    }
  });

  function onOverlayClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains("modal-overlay")) {
      handleClose();
    }
  }
</script>

{#if open}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="modal-overlay"
    onclick={onOverlayClick}
    onkeydown={(e) => { if (e.key === "Escape") handleClose(); }}
  >
    <div class="modal-panel cloud-sync-panel" role="dialog" aria-modal="true" aria-label={t("nav.cloud_sync")}>
      <div class="modal-header">
        <h3>{t("nav.cloud_sync")}</h3>
        <button class="close-btn" onclick={handleClose} aria-label={t("modal.close")}>
          &times;
        </button>
      </div>

      <div class="modal-body">
        {#if view === "status"}
          <div class="sync-status">
            {#if status}
              <div class="status-grid">
                <div class="stat-item">
                  <span class="stat-label">{t("modal.cloud_sync.entries")}</span>
                  <span class="stat-value">{status.entries}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">{t("modal.cloud_sync.uploaded")}</span>
                  <span class="stat-value">{status.uploaded_count}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">{t("modal.cloud_sync.downloaded")}</span>
                  <span class="stat-value">{status.downloaded_count}</span>
                </div>
              </div>
              {#if status.last_upload}
                <p class="last-time">{t("modal.cloud_sync.last_upload")}: {status.last_upload}</p>
              {/if}
              {#if status.last_download}
                <p class="last-time">{t("modal.cloud_sync.last_download")}: {status.last_download}</p>
              {/if}
            {:else}
              <p class="empty-hint">{t("modal.cloud_sync.empty")}</p>
            {/if}
          </div>

          <div class="sync-actions">
            <button class="sync-btn upload-btn" onclick={handleUpload}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 0a.5.5 0 01.5.5v7.793l2.146-2.147a.5.5 0 01.708.708l-3 3a.5.5 0 01-.708 0l-3-3a.5.5 0 11.708-.708L7.5 8.293V.5A.5.5 0 018 0z"/>
                <path d="M0 10a.5.5 0 01.5.5V14a.5.5 0 00.5.5h14a.5.5 0 00.5-.5v-3.5a.5.5 0 011 0V14a1.5 1.5 0 01-1.5 1.5h-14A1.5 1.5 0 010 14v-3.5A.5.5 0 010 10z"/>
              </svg>
              {t("modal.cloud_sync.upload")}
            </button>
            <button class="sync-btn download-btn" onclick={handleDownload}>
              <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 2a.5.5 0 01.5.5v7.793l2.146-2.147a.5.5 0 01.708.708l-3 3a.5.5 0 01-.708 0l-3-3a.5.5 0 11.708-.708L7.5 10.293V2.5A.5.5 0 018 2z"/>
                <path d="M0 10a.5.5 0 01.5.5V14a.5.5 0 00.5.5h14a.5.5 0 00.5-.5v-3.5a.5.5 0 011 0V14a1.5 1.5 0 01-1.5 1.5h-14A1.5 1.5 0 010 14v-3.5A.5.5 0 010 10z"/>
              </svg>
              {t("modal.cloud_sync.download")}
            </button>
          </div>

        {:else if view === "running"}
          <div class="running-state">
            <div class="spinner"></div>
            <p>{t("modal.cloud_sync.syncing")}</p>
            <button class="cancel-btn" onclick={() => abortFn?.()}>
              {t("modal.cancel")}
            </button>
          </div>

        {:else if view === "done"}
          <div class="done-state">
            <p class="done-msg">{t("modal.cloud_sync.done")}</p>
            {#if stats}
              <div class="stats-grid">
                <div class="stat-item">
                  <span class="stat-label">{t("modal.cloud_sync.total")}</span>
                  <span class="stat-value">{stats.total}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">{t("modal.cloud_sync.synced")}</span>
                  <span class="stat-value">{stats.synced}</span>
                </div>
                <div class="stat-item">
                  <span class="stat-label">{t("modal.cloud_sync.skipped")}</span>
                  <span class="stat-value">{stats.skipped}</span>
                </div>
                {#if stats.failed > 0}
                  <div class="stat-item">
                    <span class="stat-label">{t("modal.cloud_sync.failed_count")}</span>
                    <span class="stat-value error">{stats.failed}</span>
                  </div>
                {/if}
              </div>
            {/if}
            <button class="sync-btn back-btn" onclick={() => view = "status"}>
              {t("modal.cloud_sync.back")}
            </button>
          </div>

        {:else if view === "error"}
          <div class="error-state">
            <p class="error-msg">{errorMessage}</p>
            <button class="sync-btn back-btn" onclick={() => view = "status"}>
              {t("modal.cloud_sync.back")}
            </button>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .modal-panel {
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: 8px;
    width: 400px;
    max-height: 80vh;
    overflow-y: auto;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
  }
  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-default);
  }
  .modal-header h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .close-btn {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: var(--text-muted);
    padding: 0;
    line-height: 1;
  }
  .close-btn:hover {
    color: var(--text-primary);
  }
  .modal-body {
    padding: 16px;
  }

  .sync-status {
    margin-bottom: 16px;
  }
  .status-grid, .stats-grid {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    gap: 12px;
    margin-bottom: 12px;
  }
  .stat-item {
    text-align: center;
  }
  .stat-label {
    display: block;
    font-size: 11px;
    color: var(--text-muted);
    margin-bottom: 4px;
  }
  .stat-value {
    display: block;
    font-size: 20px;
    font-weight: 600;
    color: var(--text-primary);
  }
  .stat-value.error {
    color: var(--text-danger, #e74c3c);
  }
  .last-time {
    font-size: 11px;
    color: var(--text-muted);
    margin: 4px 0 0;
  }
  .empty-hint {
    text-align: center;
    color: var(--text-muted);
    font-size: 13px;
  }

  .sync-actions {
    display: flex;
    gap: 12px;
  }
  .sync-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 10px;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background: var(--bg-surface);
    color: var(--text-primary);
    font-size: 13px;
    cursor: pointer;
  }
  .sync-btn:hover {
    background: var(--bg-hover, rgba(128, 128, 128, 0.1));
  }
  .upload-btn {
    border-color: var(--accent, #4a9eff);
  }
  .download-btn {
    border-color: var(--accent, #4a9eff);
  }
  .back-btn {
    margin-top: 12px;
    width: 100%;
  }

  .running-state {
    text-align: center;
    padding: 20px 0;
  }
  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid var(--border-default);
    border-top-color: var(--accent, #4a9eff);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin: 0 auto 12px;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
  .running-state p {
    color: var(--text-muted);
    font-size: 13px;
    margin: 0 0 12px;
  }
  .cancel-btn {
    background: none;
    border: 1px solid var(--border-default);
    color: var(--text-muted);
    padding: 6px 16px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
  }
  .cancel-btn:hover {
    color: var(--text-primary);
  }

  .done-msg {
    text-align: center;
    color: var(--text-primary);
    font-weight: 500;
    margin: 0 0 12px;
  }
  .error-msg {
    text-align: center;
    color: var(--text-danger, #e74c3c);
    font-size: 13px;
    margin: 0 0 12px;
  }
</style>
