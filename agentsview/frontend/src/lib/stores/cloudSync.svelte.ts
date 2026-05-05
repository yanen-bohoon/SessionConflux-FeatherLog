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
  scanPhase: string = $state("");
  scanDetail: string = $state("");

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
    this.machines = [];
    this.scanPhase = "";
    this.scanDetail = "";
    try {
      const stream = getCloudSyncRemote((ev) => {
        switch (ev.type) {
          case "phase":
            this.scanPhase = ev.phase;
            this.scanDetail = ev.detail ?? "";
            break;
          case "machine":
            this.machines = [...this.machines, ev.machine];
            break;
        }
      });
      await stream.done;
    } catch {
      // keep whatever machines arrived before the error
    } finally {
      this.loadingMachines = false;
      this.scanPhase = "";
      this.scanDetail = "";
    }
  }

  async refresh() {
    this.loadingMachines = true;
    this.loadingStatus = true;
    await Promise.all([this.loadMachines(), this.loadStatus()]);
  }
}

export const cloudSync = new CloudSyncStore();
