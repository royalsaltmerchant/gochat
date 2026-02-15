import { relayBaseURL, officialHostUUID, officialHostName } from "../lib/config.js";

const HOSTS_KEY = "parch.hosts";

function readHosts() {
  try {
    const raw = localStorage.getItem(HOSTS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch (_err) {
    return [];
  }
}

function writeHosts(hosts) {
  localStorage.setItem(HOSTS_KEY, JSON.stringify(hosts));
}

const webPlatform = {
  async greet(name) {
    return `Hello ${name}, It's show time!`;
  },

  async alert(message) {
    window.alert(message);
  },

  async confirm(message) {
    return window.confirm(message);
  },

  async openExternal(url) {
    window.open(url, "_blank", "noopener,noreferrer");
  },

  async getHosts() {
    const hosts = readHosts();
    if (officialHostUUID && !hosts.some((h) => h.uuid === officialHostUUID)) {
      hosts.push({ uuid: officialHostUUID, name: officialHostName });
      writeHosts(hosts);
    }
    return hosts;
  },

  async verifyHostKey(hostUUID) {
    const response = await fetch(`${relayBaseURL}/api/host/${hostUUID}`);
    if (!response.ok) {
      throw new Error("host not found");
    }

    const host = await response.json();
    const hosts = readHosts();
    if (!hosts.some((h) => h.uuid === hostUUID)) {
      hosts.push({ uuid: host.uuid, name: host.name });
      writeHosts(hosts);
    }

    return host;
  },

  async removeHost(hostUUID) {
    const hosts = readHosts();
    const nextHosts = hosts.filter((host) => host.uuid !== hostUUID);
    if (nextHosts.length === hosts.length) {
      throw new Error(`host with UUID ${hostUUID} not found`);
    }
    writeHosts(nextHosts);
  },
};

export default webPlatform;
