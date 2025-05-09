import createElement from "./createElement.js";

class Invites {
  constructor() {
    this.domComponent = document.getElementById("invites");
    this.invites = [];
    this.init();
  }

  init = async () => {
    // get invites
    try {
      const res = await fetch("/api/get_invites");
      const data = await res.json();
      this.invites = data.invites;
    } catch (error) {
      console.log(error);
    }

    this.render();
  };

  acceptInvite = async (inviteID) => {
    try {
      const response = await fetch("/api/accept_invite", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ spaceUserID: String(inviteID) }),
      });
      const result = await response.json();
      if (response.ok) {
        this.invites = this.invites.filter((i) => i.ID !== inviteID);
        this.render();
      } else {
        console.log(result);
      }
    } catch (error) {
      console.log(error);
      window.alert(error);
    }
  };

  declineInvite = async (inviteID) => {
    try {
      const response = await fetch("/api/decline_invite", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ spaceUserID: String(inviteID) }),
      });
      if (response.status === 400) {
        window.alert("Error: Failed to decline invite!");
        return;
      }
      this.invites = this.invites.filter((i) => i.ID !== inviteID);
      this.render();
    } catch (error) {
      console.log(error);
    }
  };

  render = () => {
    this.domComponent.innerHTML = "";

    this.domComponent.append(
      createElement("h3", {}, "Pending Invites")
    )

    if (this.invites && this.invites.length > 0) {
      this.domComponent.append(
        createElement("div", { class: "invites-section" }, [
          ...this.invites.map((invite) =>
            createElement("div", { class: "pending-invites-item" }, [
              createElement("span", {}, invite.Name),
              createElement(
                "button",
                { class: "btn-small accept-invite" },
                "Accept",
                { type: "click", event: () => this.acceptInvite(invite.ID) }
              ),
              createElement(
                "button",
                { class: "btn-small btn-red decline-invite" },
                "Decline",
                { type: "click", event: () => this.declineInvite(invite.ID) }
              ),
            ])
          ),
        ])
      );
    }
  };
}

const invites = new Invites();
