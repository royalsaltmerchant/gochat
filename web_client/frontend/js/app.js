import createElement from "./components/createElement.js";
import DashboardApp from "./dashboard.js";

class App {
  constructor() {
    window.go.main.App.Greet("Hello GoChat").then((res) => console.log(res));

    this.domComponent = document.getElementById("app");
    this.hostForm = new HostForm(this);
    this.dashboard = new DashboardApp({
      returnToHostList: this.returnToHostList,
    });

    this.render({ type: "host_form" });
  }

  returnToHostList = () => {
    this.dashboard.currentSpaceUUID = null;
    this.dashboard.sidebar.render();
    this.dashboard.mainContent.render();
    this.dashboard.socketConn.hardClose();

    this.render({ type: "host_form" });
  };

  render = (props) => {
    this.domComponent.innerHTML = "";
    switch (props.type) {
      case "dash":
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

  renderHostList = (hosts) => {
    if (!hosts.length) {
      return [createElement("div", {}, "...No Hosts")];
    }

    return hosts.map((host) => {
      return createElement("div", { class: "host-item" }, `${host.name} `, {
        type: "click",
        event: () => {
          localStorage.setItem("hostUUID", host.uuid);
          this.app.render({ type: "dash" });
        },
      });
    });
  };

  renderKnownHosts = async () => {
    const hosts = await window.go.main.App.GetHosts();

    this.domComponent.append(
      createElement("h2", {}, "Select From Known Hosts"),
      createElement("br"),
      createElement("div", { class: "host-list-index" }, [
        ...this.renderHostList(hosts),
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

            window.go.main.App.VerifyHostKey(hostKey)
              .then((hostName) => {
                this.render({ type: "known" });
              })
              .catch((err) => {
                alert("Invalid host key");
                console.error(err);
              });
          },
        }
      ),
      createElement("br"),
      createElement("button", {}, "Return To List", {
        type: "click",
        event: () => {
          this.render({ type: "knwon" });
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
