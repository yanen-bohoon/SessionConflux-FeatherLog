/** Matches Go SyncCloudResponse in internal/server/settings.go */
export interface CloudSyncConfig {
  enabled: boolean;
  schedule: string;
  direction: string;
  compression_level: number;
  exclude_agents?: string[];
  transport: CloudSyncTransport;
}

export interface CloudSyncTransport {
  backend: string;
  feishu?: CloudSyncFeishu;
  ssh?: CloudSyncSSH;
}

export interface CloudSyncFeishu {
  app_id: string;
  app_secret: string;
  folder_token?: string;
}

export interface CloudSyncSSH {
  host: string;
  port: number;
  user: string;
  key_file: string;
  remote_path: string;
}

/** Matches Go Info in pkg/sessionconflux/sessionconflux.go */
export interface CloudSyncStatus {
  entries: number;
  uploaded_count: number;
  downloaded_count: number;
  last_upload?: string;
  last_download?: string;
}

/** Matches Go Stats in pkg/sessionconflux/sessionconflux.go */
export interface CloudSyncStats {
  total: number;
  synced: number;
  skipped: number;
  failed: number;
}

export interface CloudSyncTestResult {
  ok: boolean;
  message: string;
}
