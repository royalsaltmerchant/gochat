import { relayBaseURL } from "./lib/config.js";
import createElement from "./components/createElement.js";
import DashboardApp from "./dashboard.js";
import platform from "./platform/index.js";

class App {
  constructor() {
    platform.greet("Hello Parch").then((res) => console.log(res));

    this.domComponent = document.getElementById("app");
    this.hostForm = new HostForm(this);

    this.render({ type: "host_form" });
  }

  returnToHostList = async () => {
    // Remove host UUID
    localStorage.removeItem("hostUUID");
    // Leave socket connection
    if (this.dashboard?.socketConn) {
      this.dashboard.socketConn.hardClose();
    }
    // Set dashboard to null
    if (this.dashboard?.domComponent) {
      this.dashboard.domComponent.innerHTML = "";
    }
    this.dashboard = null;
    // Render host form
    this.render({ type: "host_form" });
  };

  render = (props) => {
    this.domComponent.innerHTML = "";
    switch (props.type) {
      case "dash":
        this.dashboard = new DashboardApp({
          returnToHostList: this.returnToHostList,
        });
        this.domComponent.append(this.dashboard.domComponent);
        this.dashboard.initialize();
        break;
      case "host_form":
        this.domComponent.append(this.hostForm.domComponent);
        this.hostForm.render({ type: "known" });
        break;
      default:
        this.domComponent.append(this.hostForm.domComponent);
        this.hostForm.render({ type: "known" });
    }
  };
}

export default class HostForm {
  constructor(app) {
    this.app = app;
    this.domComponent = createElement("div", { class: "host-form-container" });
  }

  renderHostList = (data) => {
    if (!data || !data.hosts || !data.hosts.length) {
      return [createElement("div", {}, "...No Hosts")];
    }

    const hosts = data.hosts;

    return hosts.map((host) => {
      const hostOnline = host.online == 1 ? true : false;
      return createElement(
        "div",
        { class: "host-item" },
        [
          createElement(
            "div",
            { style: "display: flex; align-items: baseline" },
            [
              createElement("span", {
                class: hostOnline ? "host-online-span" : "host-offline-span",
              }),
              createElement(
                "div",
                { class: "host-item-name" },
                `${host.name} `
              ),
            ]
          ),
          createElement("button", { class: "btn-small btn-red" }, "Remove", {
            type: "click",
            event: (event) => {
              event.stopPropagation();
              platform.confirm(
                "Are you sure you want to remove this host?"
              ).then((confirmed) => {
                if (confirmed) {
                  platform.removeHost(host.uuid);
                  event.target.parentElement.remove();
                }
              });
            },
          }),
        ],
        {
          type: "click",
          event: () => {
            if (hostOnline) {
              localStorage.setItem("hostUUID", host.uuid);
              this.app.render({ type: "dash" });
            } else platform.alert("Host is not online");
          },
        }
      );
    });
  };

  renderKnownHosts = async () => {
    const hosts = await platform.getHosts();
    const hostUUIDs = hosts.map((host) => host.uuid);

    // Get hosts by UUID from relay API
    let hostsData = [];
    try {
      const res = await fetch(`${relayBaseURL}/api/hosts_by_uuids`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ uuids: hostUUIDs }),
      });
      hostsData = await res.json();
    } catch (error) {
      console.log(error);
      platform.alert("Failed to connect to relay server");
    }

    this.domComponent.append(
      createElement("h2", {}, "Select From Known Hosts"),
      createElement("br"),
      createElement("button", {}, "Refresh", {
        type: "click",
        event: () => {
          this.render({ type: "known" });
        },
      }),
      createElement("br"),
      createElement("div", { class: "host-list-index" }, [
        ...this.renderHostList(hostsData),
      ]),
      createElement("br"),
      createElement("div", {}, "Or"),
      createElement("br"),
      createElement("button", { class: "" }, "Add New Host", {
        type: "click",
        event: () => {
          this.render({ type: "new" });
        },
      })
    );
  };

  renderNewHostForm = () => {
    this.domComponent.append(
      createElement("h2", {}, "Enter Host Key"),
      createElement(
        "form",
        {
          id: "hostForm",
          onsubmit: this.handleSubmit,
        },
        [
          createElement("fieldset", {}, [
            createElement("legend", {}, "New Host"),
            createElement("div", { class: "input-container" }, [
              createElement("label", { for: "host-key" }, "Host Key"),
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
            const hostKey = this.domComponent.querySelector("#hostKey").value;
            if (!hostKey) return false;

            platform.verifyHostKey(hostKey)
              .then((hostName) => {
                this.render({ type: "known" });
              })
              .catch((err) => {
                platform.alert("Invalid host key");

                console.error(err);
              });
          },
        }
      ),
      createElement("br"),
      createElement("button", {}, "Return To List", {
        type: "click",
        event: () => {
          this.render({ type: "known" });
        },
      })
    );
  };

  render = (props) => {
    this.domComponent.innerHTML = ""; // Clear if re-rendering

    switch (props.type) {
      case "known":
        this.renderKnownHosts();
        break;
      case "new":
        this.renderNewHostForm();
        break;
      default:
        this.renderKnownHosts();
    }
  };
}

new App();
