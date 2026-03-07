import { relayBaseURL } from "./lib/config.js";
import DashboardApp from "./dashboard.js";
import platform from "./platform/index.js";
import { Component, createElement } from "./lib/foundation.js";

class App extends Component {
  constructor() {
    const mountElem = document.getElementById("app");
    if (!mountElem) {
      throw new Error("Missing #app mount element");
    }

    super({
      domElem: mountElem,
      state: {
        view: "host_form",
      },
      autoInit: false,
      autoRender: false,
    });
    this.dashboard = null;

    platform.greet("Hello Parch").then((res) => console.log(res));

    this.render();
  }

  openDashboard = async (hostUUID) => {
    if (hostUUID) {
      localStorage.setItem("hostUUID", hostUUID);
    }
    await this.setState({ view: "dash" });
  };

  returnToHostList = async () => {
    localStorage.removeItem("hostUUID");

    this.dropChild("dashboard");
    this.dashboard = null;

    await this.setState({ view: "host_form" });

    const hostForm = this._childStore.get("host-form");
    if (hostForm && typeof hostForm.showKnownHosts === "function") {
      await hostForm.showKnownHosts();
    }
  };

  getHostForm = () => {
    return this.useChild(
      "host-form",
      () =>
        new HostForm({
          app: this,
          domElem: createElement("div", { class: "host-form-container" }),
          autoInit: false,
          autoRender: false,
        }),
      (child) => {
        child.app = this;
      }
    );
  };

  getDashboard = () => {
    return this.useChild(
      "dashboard",
      () =>
        new DashboardApp({
          returnToHostList: this.returnToHostList,
          domElem: createElement("div", { class: "dashboard-container" }),
          autoInit: false,
          autoRender: false,
        }),
      (child) => {
        child.returnToHostList = this.returnToHostList;
      }
    );
  };

  render = async () => {
    if (this.state.view === "dash") {
      const dashboard = this.getDashboard();
      this.dashboard = dashboard;
      await dashboard.init?.();
      await dashboard.render();
      return dashboard.domElem;
    }

    const hostForm = this.getHostForm();
    await hostForm.init?.();
    await hostForm.render();
    return hostForm.domElem;
  };
}

export default class HostForm extends Component {
  constructor(props) {
    super({
      domElem: props.domElem || createElement("div", { class: "host-form-container" }),
      state: {
        mode: "known",
        hostsData: [],
        loading: false,
        errorMessage: "",
        hasLoadedHosts: false,
      },
      autoInit: props.autoInit,
      autoRender: props.autoRender,
    });
    this.app = props.app;
    this._loadSequence = 0;
  }

  init = async () => {
    if (this.state.hasLoadedHosts || this.state.loading) {
      return;
    }
    await this.loadKnownHosts();
  };

  showKnownHosts = async () => {
    await this.setState({ mode: "known" });
    await this.loadKnownHosts();
  };

  showNewHostForm = async () => {
    await this.setState({ mode: "new" });
  };

  loadKnownHosts = async () => {
    const requestSequence = ++this._loadSequence;

    await this.setState(
      {
        loading: true,
        errorMessage: "",
      },
      { render: false }
    );

    let hostsData = [];

    try {
      const hosts = await platform.getHosts();

      const hostUUIDs = this.useMemo(
        "known-host-uuids",
        () => hosts.map((host) => host.uuid),
        () => [hosts.length, ...hosts.map((host) => host.uuid)]
      );

      if (hostUUIDs.length > 0) {
        const res = await fetch(`${relayBaseURL}/api/hosts_by_uuids`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ uuids: hostUUIDs }),
        });
        const payload = await res.json();
        hostsData = Array.isArray(payload?.hosts) ? payload.hosts : [];
      }
    } catch (error) {
      console.log(error);
      if (requestSequence === this._loadSequence) {
        await this.setState({
          loading: false,
          errorMessage: "Failed to connect to relay server",
          hasLoadedHosts: true,
        });
      }
      return;
    }

    if (requestSequence !== this._loadSequence) {
      return;
    }

    await this.setState({
      hostsData: Array.isArray(hostsData) ? hostsData : [],
      loading: false,
      hasLoadedHosts: true,
    });
  };

  removeHost = async (hostUUID) => {
    platform.removeHost(hostUUID);

    await this.setState((prev) => ({
      hostsData: (prev.hostsData || []).filter((host) => host.uuid !== hostUUID),
    }));
  };

  renderHostList = (hostsData) => {
    if (!Array.isArray(hostsData) || hostsData.length === 0) {
      return [createElement("div", { class: "host-empty-state" }, "No saved hosts yet.")];
    }

    const sortedHosts = this.useMemo(
      "sorted-host-list",
      () =>
        [...hostsData].sort((a, b) => {
          const aOnline = Number(a?.online) === 1 ? 1 : 0;
          const bOnline = Number(b?.online) === 1 ? 1 : 0;
          if (aOnline !== bOnline) return bOnline - aOnline;
          return String(a?.name || "").localeCompare(String(b?.name || ""));
        }),
      () => hostsData.map((host) => `${host.uuid}:${host.online}:${host.name}`)
    );

    return sortedHosts.map((host) => {
      const hostOnline = Number(host.online) === 1;

      return createElement(
        "div",
        { class: "host-item" },
        [
          createElement("div", { style: "display: flex; align-items: baseline" }, [
            createElement("span", {
              class: hostOnline ? "host-online-span" : "host-offline-span",
            }),
            createElement("div", { class: "host-item-name" }, `${host.name} `),
          ]),
          createElement("button", { class: "btn-small btn-red" }, "Remove", {
            type: "click",
            event: async (event) => {
              event.stopPropagation();
              const confirmed = await platform.confirm(
                "Are you sure you want to remove this host?"
              );
              if (confirmed) {
                await this.removeHost(host.uuid);
              }
            },
          }),
        ],
        {
          type: "click",
          event: async () => {
            if (!hostOnline) {
              platform.alert("Host is not online");
              return;
            }
            await this.app.openDashboard(host.uuid);
          },
        }
      );
    });
  };

  renderKnownHosts = () => {
    const showLoading = this.state.loading || !this.state.hasLoadedHosts;

    return [
      createElement("h2", {}, "Connect to a Host"),
      createElement(
        "p",
        { class: "host-screen-subtitle" },
        "Choose a saved host or add a new host key. Relay access is granted only after host challenge auth and per-space capability checks."
      ),
      createElement("br"),
      createElement("button", {}, "Refresh", {
        type: "click",
        event: () => {
          this.loadKnownHosts();
        },
      }),
      createElement("br"),
      this.state.errorMessage
        ? createElement("div", { class: "host-empty-state" }, this.state.errorMessage)
        : createElement("div", { class: "host-list-index" }, [
            ...(showLoading
              ? [createElement("div", { class: "host-empty-state" }, "Loading hosts...")]
              : this.renderHostList(this.state.hostsData)),
          ]),
      createElement("br"),
      createElement("div", { class: "host-divider-text" }, "Or"),
      createElement("br"),
      createElement("button", {}, "Add Host Key", {
        type: "click",
        event: () => {
          this.showNewHostForm();
        },
      }),
    ];
  };

  renderNewHostForm = () => {
    return [
      createElement("h2", {}, "Add Host Key"),
      createElement(
        "p",
        { class: "host-screen-subtitle" },
        "Paste the key shared by your host operator. You will only connect if that host is online and authenticated."
      ),
      createElement(
        "form",
        {
          id: "hostForm",
        },
        [
          createElement("fieldset", {}, [
            createElement("legend", {}, "Host Connection"),
            createElement("div", { class: "input-container" }, [
              createElement("label", { for: "hostKey" }, "Host Access Key"),
              createElement("input", {
                type: "text",
                id: "hostKey",
                name: "hostKey",
                required: true,
              }),
            ]),
            createElement("br"),
            createElement("button", { type: "submit" }, "Continue"),
          ]),
        ],
        {
          type: "submit",
          event: (event) => {
            event.preventDefault();
            const hostKey = this.domElem.querySelector("#hostKey")?.value;
            if (!hostKey) return;

            platform
              .verifyHostKey(hostKey)
              .then(async () => {
                await this.showKnownHosts();
              })
              .catch((err) => {
                platform.alert("Invalid host key");
                console.error(err);
              });
          },
        }
      ),
      createElement("br"),
      createElement("button", {}, "Back To Hosts", {
        type: "click",
        event: () => {
          this.showKnownHosts();
        },
      }),
    ];
  };

  render = async () => {
    if (this.state.mode === "known" && !this.state.hasLoadedHosts && !this.state.loading) {
      this.loadKnownHosts();
    }

    if (this.state.mode === "new") {
      return this.renderNewHostForm();
    }

    return this.renderKnownHosts();
  };
}

new App();
