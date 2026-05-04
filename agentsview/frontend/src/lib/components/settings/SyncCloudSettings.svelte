<script lang="ts">
  import SettingsSection from "./SettingsSection.svelte";
  import { settings } from "../../stores/settings.svelte.js";
  import { t } from "../../i18n/index.js";
  import {
    updateSettings,
    testCloudSyncConnection,
    type CloudSyncConfig,
  } from "../../api/client.js";

  let enabled: boolean = $state(false);
  let schedule: string = $state("02:00");
  let direction: string = $state("both");
  let backend: string = $state("feishu");
  let compressionLevel: number = $state(3);

  // Feishu
  let feishuAppId: string = $state("");
  let feishuAppSecret: string = $state("");
  let feishuFolderToken: string = $state("");

  // SSH
  let sshHost: string = $state("");
  let sshPort: number = $state(22);
  let sshUser: string = $state("");
  let sshKeyFile: string = $state("");
  let sshRemotePath: string = $state("");

  let saving: boolean = $state(false);
  let testing: boolean = $state(false);
  let error: string | null = $state(null);
  let success: string | null = $state(null);
  let testResult: string | null = $state(null);

  // Populate from loaded settings.
  $effect(() => {
    const sc = settings.syncCloud;
    if (!sc) return;
    enabled = sc.enabled;
    if (sc.schedule) schedule = sc.schedule;
    if (sc.direction) direction = sc.direction;
    if (sc.compression_level) compressionLevel = sc.compression_level;
    if (sc.transport) {
      if (sc.transport.backend) backend = sc.transport.backend;
      if (sc.transport.feishu) {
        if (sc.transport.feishu.app_id) feishuAppId = sc.transport.feishu.app_id;
        if (sc.transport.feishu.app_secret) feishuAppSecret = sc.transport.feishu.app_secret;
        if (sc.transport.feishu.folder_token) feishuFolderToken = sc.transport.feishu.folder_token;
      }
      if (sc.transport.ssh) {
        if (sc.transport.ssh.host) sshHost = sc.transport.ssh.host;
        if (sc.transport.ssh.port) sshPort = sc.transport.ssh.port;
        if (sc.transport.ssh.user) sshUser = sc.transport.ssh.user;
        if (sc.transport.ssh.key_file) sshKeyFile = sc.transport.ssh.key_file;
        if (sc.transport.ssh.remote_path) sshRemotePath = sc.transport.ssh.remote_path;
      }
    }
  });

  function buildSyncCloudPatch(): CloudSyncConfig {
    return {
      enabled,
      schedule,
      direction,
      compression_level: compressionLevel,
      transport: {
        backend,
        feishu: {
          app_id: feishuAppId,
          app_secret: feishuAppSecret,
          folder_token: feishuFolderToken || undefined,
        },
        ssh: {
          host: sshHost,
          port: sshPort,
          user: sshUser,
          key_file: sshKeyFile,
          remote_path: sshRemotePath,
        },
      },
    };
  }

  async function handleSave() {
    saving = true;
    error = null;
    success = null;
    testResult = null;
    try {
      await updateSettings({ sync_cloud: buildSyncCloudPatch() } as any);
      success = t("settings.saved");
    } catch (e) {
      error = e instanceof Error ? e.message : t("settings.save_error");
    } finally {
      saving = false;
    }
  }

  async function handleTest() {
    testing = true;
    error = null;
    success = null;
    testResult = null;
    // Save first so the server has the latest config for testing.
    try {
      await updateSettings({ sync_cloud: buildSyncCloudPatch() } as any);
    } catch {
      // continue anyway
    }
    try {
      const result = await testCloudSyncConnection();
      testResult = result.message;
    } catch (e) {
      testResult = e instanceof Error ? e.message : t("settings.sync_cloud.test_failed");
    } finally {
      testing = false;
    }
  }
</script>

<SettingsSection
  title={t("settings.sync_cloud.title")}
  description={t("settings.sync_cloud.desc")}
>
  <!-- Enable toggle -->
  <label class="toggle-row">
    <span class="toggle-label">{t("settings.sync_cloud.enabled")}</span>
    <input type="checkbox" bind:checked={enabled} />
  </label>

  <!-- Schedule -->
  <div class="field-row">
    <label class="field-label" for="sync-schedule">{t("settings.sync_cloud.schedule")}</label>
    <input
      id="sync-schedule"
      class="setting-input short-input"
      type="text"
      bind:value={schedule}
      placeholder="02:00"
    />
  </div>

  <!-- Direction -->
  <div class="field-row">
    <label class="field-label" for="sync-direction">{t("settings.sync_cloud.direction")}</label>
    <select id="sync-direction" class="setting-select" bind:value={direction}>
      <option value="both">{t("settings.sync_cloud.direction_both")}</option>
      <option value="upload">{t("settings.sync_cloud.direction_upload")}</option>
      <option value="download">{t("settings.sync_cloud.direction_download")}</option>
    </select>
  </div>

  <!-- Compression -->
  <div class="field-row">
    <label class="field-label" for="sync-compress">{t("settings.sync_cloud.compression")}</label>
    <input
      id="sync-compress"
      class="setting-input short-input"
      type="number"
      min="1"
      max="22"
      bind:value={compressionLevel}
    />
  </div>

  <!-- Transport backend -->
  <div class="field-row">
    <label class="field-label" for="sync-backend">{t("settings.sync_cloud.transport")}</label>
    <select id="sync-backend" class="setting-select" bind:value={backend}>
      <option value="feishu">{t("settings.sync_cloud.feishu")}</option>
      <option value="ssh">{t("settings.sync_cloud.ssh")}</option>
    </select>
  </div>

  <!-- Feishu fields -->
  {#if backend === "feishu"}
    <div class="sub-fields">
      <div class="field-row">
        <label class="field-label" for="sync-feishu-id">{t("settings.sync_cloud.feishu_app_id")}</label>
        <input id="sync-feishu-id" class="setting-input" type="text" bind:value={feishuAppId} />
      </div>
      <div class="field-row">
        <label class="field-label" for="sync-feishu-secret">{t("settings.sync_cloud.feishu_secret")}</label>
        <input id="sync-feishu-secret" class="setting-input" type="password" bind:value={feishuAppSecret} />
      </div>
      <div class="field-row">
        <label class="field-label" for="sync-feishu-folder">{t("settings.sync_cloud.feishu_folder")}</label>
        <input id="sync-feishu-folder" class="setting-input" type="text" bind:value={feishuFolderToken} placeholder={t("settings.sync_cloud.optional")} />
      </div>
    </div>
  {/if}

  <!-- SSH fields -->
  {#if backend === "ssh"}
    <div class="sub-fields">
      <div class="field-row">
        <label class="field-label" for="sync-ssh-host">{t("settings.sync_cloud.ssh_host")}</label>
        <input id="sync-ssh-host" class="setting-input" type="text" bind:value={sshHost} placeholder="192.168.1.100" />
      </div>
      <div class="field-row">
        <label class="field-label" for="sync-ssh-port">{t("settings.sync_cloud.ssh_port")}</label>
        <input id="sync-ssh-port" class="setting-input short-input" type="number" bind:value={sshPort} min="1" max="65535" />
      </div>
      <div class="field-row">
        <label class="field-label" for="sync-ssh-user">{t("settings.sync_cloud.ssh_user")}</label>
        <input id="sync-ssh-user" class="setting-input" type="text" bind:value={sshUser} />
      </div>
      <div class="field-row">
        <label class="field-label" for="sync-ssh-key">{t("settings.sync_cloud.ssh_key")}</label>
        <input id="sync-ssh-key" class="setting-input" type="text" bind:value={sshKeyFile} placeholder="~/.ssh/id_rsa" />
      </div>
      <div class="field-row">
        <label class="field-label" for="sync-ssh-path">{t("settings.sync_cloud.ssh_path")}</label>
        <input id="sync-ssh-path" class="setting-input" type="text" bind:value={sshRemotePath} placeholder="/home/user/agentsview-bundles" />
      </div>
    </div>
  {/if}

  <!-- Actions -->
  <div class="actions-row">
    <button class="save-btn" disabled={saving} onclick={handleSave}>
      {saving ? t("settings.saving") : t("settings.save")}
    </button>
    <button class="test-btn" disabled={testing} onclick={handleTest}>
      {testing ? t("settings.sync_cloud.testing") : t("settings.sync_cloud.test")}
    </button>
  </div>

  {#if testResult}
    <p class="msg test-msg">{testResult}</p>
  {/if}
  {#if error}
    <p class="msg error">{error}</p>
  {/if}
  {#if success}
    <p class="msg success">{success}</p>
  {/if}
</SettingsSection>

<style>
  /* Every config row uses a fixed label column so inputs align vertically.
     The label column is 90px; the input/select/checkbox fills remaining space. */
  .field-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .field-label {
    width: 90px;
    flex-shrink: 0;
    font-size: 11px;
    font-weight: 500;
    color: var(--text-muted);
    text-align: right;
  }

  /* Toggle row shares the same label-input rhythm as field rows. */
  .toggle-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .toggle-label {
    width: 90px;
    flex-shrink: 0;
    font-size: 12px;
    font-weight: 500;
    color: var(--text-secondary);
    text-align: right;
  }
  .toggle-row input[type="checkbox"] {
    width: 16px;
    height: 16px;
    accent-color: var(--accent-blue);
  }

  .setting-input {
    flex: 1;
    height: 28px;
    padding: 0 8px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-primary);
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
  }
  .setting-input:focus {
    outline: none;
    border-color: var(--accent-blue);
  }
  .short-input {
    flex: 1;
    max-width: 100px;
  }
  .setting-select {
    flex: 1;
    height: 28px;
    padding: 0 6px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-primary);
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
  }

  /* Sub-fields are visually nested. Their labels are narrower to
     keep the total label+edge width aligned with the parent labels. */
  .sub-fields {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 0 8px 16px;
    border-left: 2px solid var(--border-muted);
    margin-left: 12px;
  }
  .sub-fields .field-label {
    width: 60px;
  }

  .actions-row {
    display: flex;
    gap: 8px;
    padding-top: 4px;
    padding-left: 98px;
  }
  .save-btn {
    height: 28px;
    padding: 0 14px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-weight: 500;
    color: white;
    background: var(--accent-blue);
    border: none;
    cursor: pointer;
  }
  .save-btn:hover:not(:disabled) {
    opacity: 0.9;
  }
  .save-btn:disabled {
    opacity: 0.6;
    cursor: default;
  }
  .test-btn {
    height: 28px;
    padding: 0 14px;
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-weight: 500;
    color: var(--text-primary);
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
    cursor: pointer;
  }
  .test-btn:hover:not(:disabled) {
    border-color: var(--text-muted);
  }
  .test-btn:disabled {
    opacity: 0.6;
    cursor: default;
  }

  .msg {
    font-size: 11px;
    margin: 0;
    padding-left: 98px;
  }
  .msg.error {
    color: var(--accent-red, #ef4444);
  }
  .msg.success {
    color: var(--accent-green, #22c55e);
  }
  .msg.test-msg {
    color: var(--text-secondary);
  }
</style>
