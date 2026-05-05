<script lang="ts">
  import {
    getCloudSyncStatus,
    getCloudSyncRemote,
    uploadCloudSync,
    downloadCloudSync,
    deleteCloudSyncRemote,
    type CloudSyncStatus,
    type CloudSyncStats,
    type CloudSyncEvent,
  } from "../../api/client.js";
  import type { CloudSyncMachine } from "../../api/types.js";
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

  type View = "explorer" | "running" | "done" | "error";

  let view: View = $state("explorer");
  let errorMessage: string = $state("");
  let status: CloudSyncStatus | null = $state(null);
  let stats: CloudSyncStats | null = $state(null);
  let operation: string = $state("");
  let abortFn: (() => void) | null = $state(null);
  let progressPhase: string = $state("");
  let progressCurrent: number = $state(0);
  let progressTotal: number = $state(0);
  let progressDetail: string = $state("");
  let machines: CloudSyncMachine[] = $state([]);
  let loadingMachines: boolean = $state(false);
  let downloadingHost: string = $state("");
  let deletingHost: string = $state("");
  let confirmDelete: string = $state("");

  async function loadStatus() {
    try {
      status = await getCloudSyncStatus();
    } catch {
      // status unavailable — show empty
    }
  }

  async function loadMachines() {
    loadingMachines = true;
    try {
      const resp = await getCloudSyncRemote();
      machines = resp.machines ?? [];
    } catch {
      machines = [];
    } finally {
      loadingMachines = false;
    }
  }

  function handleEvent(ev: CloudSyncEvent) {
    switch (ev.type) {
      case "started":
        operation = ev.operation;
        progressPhase = "";
        progressCurrent = 0;
        progressTotal = 0;
        progressDetail = "";
        break;
      case "progress":
        progressPhase = ev.phase;
        progressCurrent = ev.current;
        progressTotal = ev.total;
        progressDetail = ev.detail;
        break;
      case "done":
        if (deletingHost) {
          deletingHost = "";
          confirmDelete = "";
          loadStatus();
          loadMachines();
          view = "explorer";
        } else {
          stats = ev.stats;
          view = "done";
          downloadingHost = "";
          loadStatus();
          onsynced();
        }
        break;
      case "error":
        errorMessage = ev.message;
        view = "error";
        downloadingHost = "";
        deletingHost = "";
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
    } catch {
      if (view === "running") {
        errorMessage = t("modal.cloud_sync.failed");
        view = "error";
      }
    }
    abortFn = null;
  }

  async function handleDownload(hostname?: string) {
    view = "running";
    downloadingHost = hostname ?? "";
    errorMessage = "";
    stats = null;
    const stream = downloadCloudSync(handleEvent, hostname);
    abortFn = () => stream.abort();
    try {
      await stream.done;
    } catch {
      if (view === "running") {
        errorMessage = t("modal.cloud_sync.failed");
        view = "error";
      }
    }
    abortFn = null;
  }

  async function handleDelete(hostname: string) {
    view = "running";
    deletingHost = hostname;
    errorMessage = "";
    stats = null;
    const stream = deleteCloudSyncRemote(hostname, handleEvent);
    abortFn = () => stream.abort();
    try {
      await stream.done;
    } catch {
      if (view === "running") {
        errorMessage = t("modal.cloud_sync.failed");
        view = "error";
      }
    }
    abortFn = null;
  }

  function handleClose() {
    if (view === "running") return;
    view = "explorer";
    errorMessage = "";
    stats = null;
    downloadingHost = "";
    deletingHost = "";
    confirmDelete = "";
    open = false;
    onclose();
  }

  // Load data when modal opens.
  $effect(() => {
    if (open) {
      view = "explorer";
      errorMessage = "";
      stats = null;
      downloadingHost = "";
      deletingHost = "";
      confirmDelete = "";
      loadStatus();
      loadMachines();
    }
  });

  function onOverlayClick(e: MouseEvent) {
    if ((e.target as HTMLElement).classList.contains("modal-overlay")) {
      handleClose();
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
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
        {#if view === "explorer"}
          <div class="explorer-view">
            <!-- Local machine section -->
            <div class="local-section">
              <div class="section-header">
                <span class="section-title">{t("modal.cloud_sync.local_machine")}</span>
              </div>
              <div class="local-card">
                {#if status}
                  <div class="local-stats">
                    <span class="local-stat">{t("modal.cloud_sync.entries")}: <strong>{status.entries}</strong></span>
                    <span class="local-stat">{t("modal.cloud_sync.uploaded")}: <strong>{status.uploaded_count}</strong></span>
                    <span class="local-stat">{t("modal.cloud_sync.downloaded")}: <strong>{status.downloaded_count}</strong></span>
                  </div>
                {:else}
                  <p class="empty-hint">{t("modal.cloud_sync.empty")}</p>
                {/if}
                <button class="upload-btn" onclick={handleUpload}>
                  <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M8 0a.5.5 0 01.5.5v7.793l2.146-2.147a.5.5 0 01.708.708l-3 3a.5.5 0 01-.708 0l-3-3a.5.5 0 11.708-.708L7.5 8.293V.5A.5.5 0 018 0z"/>
                    <path d="M0 10a.5.5 0 01.5.5V14a.5.5 0 00.5.5h14a.5.5 0 00.5-.5v-3.5a.5.5 0 011 0V14a1.5 1.5 0 01-1.5 1.5h-14A1.5 1.5 0 010 14v-3.5A.5.5 0 010 10z"/>
                  </svg>
                  {t("modal.cloud_sync.upload")}
                </button>
              </div>
            </div>

            <!-- Remote machines section -->
            <div class="remote-section">
              <div class="section-header">
                <span class="section-title">{t("modal.cloud_sync.remote_machines")}</span>
                <button class="refresh-btn" onclick={loadMachines} disabled={loadingMachines} aria-label={t("modal.cloud_sync.refresh")}>
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" class:spin={loadingMachines}>
                    <path d="M11.534 7.5A3.5 3.5 0 008 4.5 3.5 3.5 0 004.5 8a.5.5 0 01-1 0A4.5 4.5 0 018 3.5a4.5 4.5 0 014.465 4h1.881a.25.25 0 01.192.41l-2.692 3.536a.25.25 0 01-.384 0L8.77 7.91a.25.25 0 01.192-.41H11.534z"/>
                    <path fill-rule="evenodd" d="M8 1.5a6.5 6.5 0 100 13 6.5 6.5 0 000-13zM.5 8a7.5 7.5 0 1115 0A7.5 7.5 0 01.5 8z"/>
                  </svg>
                </button>
              </div>

              {#if loadingMachines}
                <p class="loading-hint">{t("modal.cloud_sync.loading")}</p>
              {:else if machines.length === 0}
                <p class="empty-hint">{t("modal.cloud_sync.no_remote")}</p>
              {:else}
                <div class="machine-list">
                  {#each machines as m}
                    <div class="machine-card">
                      <div class="machine-info">
                        <span class="machine-name">{m.name}</span>
                        <div class="machine-tags">
                          {#if m.baseline}
                            <span class="tag baseline-tag">
                              baseline: {m.baseline.files} files, {formatBytes(m.baseline.size)}
                            </span>
                          {/if}
                          {#if m.incremental}
                            {#each m.incremental as inc}
                              <span class="tag inc-tag">{inc.agent}: {inc.count}</span>
                            {/each}
                          {/if}
                        </div>
                      </div>
                      <div class="machine-actions">
                        <button
                          class="machine-download-btn"
                          onclick={() => handleDownload(m.name)}
                          disabled={view === "running"}
                        >
                          <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                            <path d="M8 2a.5.5 0 01.5.5v7.793l2.146-2.147a.5.5 0 01.708.708l-3 3a.5.5 0 01-.708 0l-3-3a.5.5 0 11.708-.708L7.5 10.293V2.5A.5.5 0 018 2z"/>
                            <path d="M0 10a.5.5 0 01.5.5V14a.5.5 0 00.5.5h14a.5.5 0 00.5-.5v-3.5a.5.5 0 011 0V14a1.5 1.5 0 01-1.5 1.5h-14A1.5 1.5 0 010 14v-3.5A.5.5 0 010 10z"/>
                          </svg>
                          {t("modal.cloud_sync.download")}
                        </button>
                        <button
                          class="machine-delete-btn"
                          onclick={() => confirmDelete = m.name}
                          disabled={view === "running"}
                          title={t("modal.cloud_sync.delete_machine")}
                        >
                          &times;
                        </button>
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>

            <!-- Download all bar -->
            {#if machines.length > 0}
              <div class="download-all-bar">
                <button
                  class="download-all-btn"
                  onclick={() => handleDownload()}
                  disabled={view === "running"}
                >
                  <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M8 2a.5.5 0 01.5.5v7.793l2.146-2.147a.5.5 0 01.708.708l-3 3a.5.5 0 01-.708 0l-3-3a.5.5 0 11.708-.708L7.5 10.293V2.5A.5.5 0 018 2z"/>
                    <path d="M0 10a.5.5 0 01.5.5V14a.5.5 0 00.5.5h14a.5.5 0 00.5-.5v-3.5a.5.5 0 011 0V14a1.5 1.5 0 01-1.5 1.5h-14A1.5 1.5 0 010 14v-3.5A.5.5 0 010 10z"/>
                  </svg>
                  {t("modal.cloud_sync.download_all")}
                </button>
              </div>
            {/if}

            <!-- Delete confirmation prompt -->
            {#if confirmDelete}
              <div class="confirm-overlay">
                <div class="confirm-box">
                  <p class="confirm-text">{t("modal.cloud_sync.delete_confirm", { name: confirmDelete })}</p>
                  <div class="confirm-actions">
                    <button class="confirm-cancel-btn" onclick={() => confirmDelete = ""}>
                      {t("modal.cancel")}
                    </button>
                    <button class="confirm-delete-btn" onclick={() => handleDelete(confirmDelete)}>
                      {t("modal.cloud_sync.delete_machine")}
                    </button>
                  </div>
                </div>
              </div>
            {/if}
          </div>

        {:else if view === "running"}
          <div class="running-state">
            <div class="spinner"></div>
            <p class="phase-label">
              {progressPhase
                ? t("modal.cloud_sync.phase_" + progressPhase, progressPhase)
                : t("modal.cloud_sync.syncing")}
            </p>
            {#if downloadingHost}
              <p class="download-host">{downloadingHost}</p>
            {/if}
            {#if deletingHost}
              <p class="deleting-host">{t("modal.cloud_sync.deleting", { name: deletingHost })}</p>
            {/if}
            {#if progressTotal > 0}
              <div class="progress-bar">
                <div
                  class="progress-fill"
                  style="width: {Math.min(progressCurrent / progressTotal * 100, 100)}%"
                ></div>
              </div>
              <p class="progress-count">
                {t("modal.cloud_sync.progress", { current: String(progressCurrent), total: String(progressTotal) })}
              </p>
            {/if}
            {#if progressDetail}
              <p class="progress-detail">{progressDetail}</p>
            {/if}
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
            <button class="sync-btn back-btn" onclick={() => view = "explorer"}>
              {t("modal.cloud_sync.back")}
            </button>
          </div>

        {:else if view === "error"}
          <div class="error-state">
            <p class="error-msg">{errorMessage}</p>
            <button class="sync-btn back-btn" onclick={() => view = "explorer"}>
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
    width: 480px;
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

  /* Explorer view */
  .explorer-view {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }
  .section-title {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  /* Local section */
  .local-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background: var(--bg-surface);
  }
  .local-stats {
    display: flex;
    gap: 16px;
    font-size: 12px;
    color: var(--text-muted);
  }
  .local-stat strong {
    color: var(--text-primary);
    font-weight: 600;
  }
  .upload-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 7px 14px;
    border: 1px solid var(--accent, #4a9eff);
    border-radius: 5px;
    background: var(--bg-surface);
    color: var(--accent, #4a9eff);
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
    white-space: nowrap;
  }
  .upload-btn:hover {
    background: var(--bg-hover, rgba(128, 128, 128, 0.1));
  }

  .refresh-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background: var(--bg-surface);
    color: var(--text-muted);
    cursor: pointer;
    padding: 0;
  }
  .refresh-btn:hover {
    color: var(--text-primary);
  }
  .refresh-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .spin {
    animation: spin 0.8s linear infinite;
  }

  /* Machine list */
  .machine-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .machine-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border: 1px solid var(--border-default);
    border-radius: 6px;
    background: var(--bg-surface);
    gap: 12px;
  }
  .machine-info {
    flex: 1;
    min-width: 0;
  }
  .machine-name {
    display: block;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 4px;
  }
  .machine-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }
  .tag {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 500;
    white-space: nowrap;
  }
  .baseline-tag {
    background: rgba(74, 158, 255, 0.12);
    color: var(--accent, #4a9eff);
  }
  .inc-tag {
    background: rgba(128, 128, 128, 0.12);
    color: var(--text-muted);
  }
  .machine-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }
  .machine-download-btn {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 10px;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background: var(--bg-surface);
    color: var(--text-primary);
    font-size: 11px;
    cursor: pointer;
    white-space: nowrap;
  }
  .machine-download-btn:hover {
    background: var(--bg-hover, rgba(128, 128, 128, 0.1));
  }
  .machine-download-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .machine-delete-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background: var(--bg-surface);
    color: var(--text-muted);
    font-size: 15px;
    line-height: 1;
    cursor: pointer;
    padding: 0;
  }
  .machine-delete-btn:hover {
    color: var(--text-danger, #e74c3c);
    border-color: var(--text-danger, #e74c3c);
  }
  .machine-delete-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Download all bar */
  .download-all-bar {
    padding-top: 4px;
    border-top: 1px solid var(--border-default);
  }
  .download-all-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    width: 100%;
    padding: 10px;
    border: 1px solid var(--accent, #4a9eff);
    border-radius: 6px;
    background: var(--accent, #4a9eff);
    color: #fff;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
  }
  .download-all-btn:hover {
    opacity: 0.9;
  }
  .download-all-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Hints */
  .empty-hint {
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
    margin: 8px 0;
  }
  .loading-hint {
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
    margin: 8px 0;
  }

  /* Running / Done / Error states */
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
  .phase-label {
    color: var(--text-primary);
    font-size: 13px;
    font-weight: 500;
    margin: 0 0 4px;
  }
  .download-host {
    color: var(--text-muted);
    font-size: 11px;
    margin: 0 0 10px;
  }
  .deleting-host {
    color: var(--text-danger, #e74c3c);
    font-size: 12px;
    font-weight: 500;
    margin: 0 0 10px;
  }
  .progress-bar {
    width: 100%;
    height: 6px;
    background: var(--border-default);
    border-radius: 3px;
    overflow: hidden;
    margin: 0 0 8px;
  }
  .progress-fill {
    height: 100%;
    background: var(--accent, #4a9eff);
    border-radius: 3px;
    transition: width 0.3s ease;
  }
  .progress-count {
    color: var(--text-muted);
    font-size: 12px;
    margin: 0 0 4px;
  }
  .progress-detail {
    color: var(--text-muted);
    font-size: 11px;
    margin: 0 0 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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

  .stats-grid {
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
  .back-btn {
    margin-top: 12px;
    width: 100%;
  }

  /* Confirm dialog */
  .confirm-overlay {
    margin-top: 8px;
    padding: 12px;
    border: 1px solid var(--text-danger, #e74c3c);
    border-radius: 6px;
    background: rgba(231, 76, 60, 0.06);
  }
  .confirm-text {
    font-size: 13px;
    color: var(--text-primary);
    margin: 0 0 10px;
    text-align: center;
  }
  .confirm-actions {
    display: flex;
    gap: 8px;
    justify-content: center;
  }
  .confirm-cancel-btn {
    padding: 5px 14px;
    border: 1px solid var(--border-default);
    border-radius: 4px;
    background: var(--bg-surface);
    color: var(--text-muted);
    font-size: 12px;
    cursor: pointer;
  }
  .confirm-cancel-btn:hover {
    color: var(--text-primary);
  }
  .confirm-delete-btn {
    padding: 5px 14px;
    border: 1px solid var(--text-danger, #e74c3c);
    border-radius: 4px;
    background: var(--text-danger, #e74c3c);
    color: #fff;
    font-size: 12px;
    font-weight: 500;
    cursor: pointer;
  }
  .confirm-delete-btn:hover {
    opacity: 0.9;
  }
</style>
