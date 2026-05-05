import {
  getCloudSyncStatus,
  getCloudSyncRemote,
  type CloudSyncStatus,
} from "../api/client.js";
import type { CloudSyncMachine } from "../api/types/sync-cloud.js";

class CloudSyncStore {
  status: CloudSyncStatus | null = $state(null);
  machines: CloudSyncMachine[] = $state([]);
  loadingMachines: boolean = $state(false);
  loadingStatus: boolean = $state(false);
  loaded: boolean = $state(false);

  async loadAll() {
    if (this.loaded) return;
    this.loadingMachines = true;
    this.loadingStatus = true;
    await Promise.all([this.loadMachines(), this.loadStatus()]);
    this.loaded = true;
  }

  async loadStatus() {
    this.loadingStatus = true;
    try {
      this.status = await getCloudSyncStatus();
    } catch {
      this.status = null;
    } finally {
      this.loadingStatus = false;
    }
  }

  async loadMachines() {
    this.loadingMachines = true;
    try {
      const resp = await getCloudSyncRemote();
      this.machines = resp.machines ?? [];
    } catch {
      this.machines = [];
    } finally {
      this.loadingMachines = false;
    }
  }

  async refresh() {
    this.loadingMachines = true;
    this.loadingStatus = true;
    await Promise.all([this.loadMachines(), this.loadStatus()]);
  }
}

export const cloudSync = new CloudSyncStore();
