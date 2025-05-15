import createElement from "./createElement.js";

export default class DashModal {
  constructor(app, socketConn) {
    this.app = app;
    this.socketConn = socketConn;
    this.domComponent = createElement("div", { class: "modal" });
  }

  open = (props) => {
    this.domComponent.style.display = "block";
    this.render(props);
  };

  close = () => {
    this.domComponent.style.display = "none";
  };

  renderAuthorSettings = (space) => {
    return [
      // Space Management Section
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Space Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn" }, "Invite User", {
            type: "click",
            event: () => {
              const invitedUserEmail = window
                .prompt("Please enter email address of user")
                .trim();
              if (!invitedUserEmail) return;
              this.socketConn.inviteUser({
                email: invitedUserEmail,
                space_uuid: space.uuid,
              });
            },
          }),
          createElement("button", { class: "btn btn-red" }, "Delete Space", {
            type: "click",
            event: () => {
              this.socketConn.deleteSpace({ uuid: space.uuid });
            },
          }),
        ]),
      ]),
      // Channel Management Section
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Channel Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn" }, "+ Create Channel", {
            type: "click",
            event: () => {
              const channelName = window
                .prompt("Please enter Channel 'Name'")
                .trim();
              if (!channelName) return;
              //
              this.socketConn.createChannel({
                name: channelName,
                space_uuid: space.uuid,
              });
            },
          }),
        ]),
        createElement(
          "div",
          { class: "channels-list" },
          space.channels.map((channel) =>
            createElement("div", { class: "channel-item" }, [
              createElement("span", {}, channel.name),
              createElement(
                "button",
                { class: "btn-small btn-red" },
                "Delete",
                {
                  type: "click",
                  event: () => {
                    if (
                      !window.confirm(
                        "Are you sure you want to delete this channel? This action cannot be undone."
                      )
                    ) {
                      return;
                    }
                    this.socketConn.deleteChannel({ uuid: channel.uuid });
                  },
                }
              ),
            ])
          )
        ),
      ]),
    ];
  };

  renderSpaceUserSettings = (space, user) => {
    return [
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Space Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn btn-red" }, "Leave Space", {
            type: "click",
            event: () => {
              this.socketConn.leaveSpace({
                space_uuid: space.uuid,
                user_id: user.id,
              });
            },
          }),
        ]),
      ]),
    ];
  };

  renderLoginForm = () => {
    this.domComponent.append(
      createElement("div", { class: "modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, `Login`),
          // createElement("button", { class: "close-modal" }, "×", {
          //   type: "click",
          //   event: this.close,
          // }),
        ]),
        createElement("div", { class: "modal-body" }, [
          createElement("div", {
            id: "toast-message",
            class: "toast-warning",
          }),

          createElement(
            "form",
            { id: "login-form" },
            [
              createElement("fieldset", {}, [
                createElement("legend", {}, "Login"),

                createElement("div", { class: "input-container" }, [
                  createElement("label", { for: "email" }, "Email"),
                  createElement("input", {
                    name: "email",
                    type: "email",
                    required: true,
                    autofocus: true,
                  }),
                ]),
                createElement("br"),
                createElement("div", { class: "input-container" }, [
                  createElement("label", { for: "password" }, "Password"),
                  createElement("input", {
                    name: "password",
                    type: "password",
                    required: true,
                  }),
                ]),
                createElement("br"),
                createElement("button", { type: "submit" }, "Submit"),
              ]),
            ],
            {
              type: "submit",
              event: (e) => {
                e.preventDefault();

                const form = e.target;
                const formData = new FormData(form);
                const jsonData = Object.fromEntries(formData.entries());
                this.socketConn.loginUser(jsonData);
              },
            }
          ),
          createElement("br"),
          createElement("a", {}, "Register", {
            type: "click",
            event: (e) => {
              e.preventDefault();

              this.render({
                type: "register",
                data: {},
              });
            },
          }),
        ]),
      ])
    );
  };

  renderRegisterForm = () => {
    this.domComponent.append(
      createElement("div", { class: "modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, `Register`),
          // createElement("button", { class: "close-modal" }, "×", {
          //   type: "click",
          //   event: this.close,
          // }),
        ]),
        createElement("div", { class: "modal-body" }, [
          createElement("div", {
            id: "toast-message",
            class: "toast-warning",
          }),

          createElement(
            "form",
            { id: "register-form" },
            [
              createElement("fieldset", {}, [
                createElement("legend", {}, "Register"),

                createElement("div", { class: "input-container" }, [
                  createElement("label", { for: "username" }, "Username"),
                  createElement("input", {
                    name: "username",
                    type: "text",
                    required: true,
                    autofocus: true,
                  }),
                ]),
                createElement("div", { class: "input-container" }, [
                  createElement("label", { for: "email" }, "Email"),
                  createElement("input", {
                    name: "email",
                    type: "email",
                    required: true,
                    autofocus: true,
                  }),
                ]),
                createElement("br"),
                createElement("div", { class: "input-container" }, [
                  createElement("label", { for: "password" }, "Password"),
                  createElement("input", {
                    name: "password",
                    type: "password",
                    required: true,
                  }),
                ]),
                createElement("br"),
                createElement("button", { type: "submit" }, "Submit"),
              ]),
            ],
            {
              type: "submit",
              event: (e) => {
                e.preventDefault();

                const form = e.target;
                const formData = new FormData(form);
                const jsonData = Object.fromEntries(formData.entries());
                this.socketConn.registerUser(jsonData);
              },
            }
          ),
          createElement("br"),
          createElement("a", {}, "Login", {
            type: "click",
            event: (e) => {
              e.preventDefault();

              this.render({
                type: "login",
                data: {},
              });
            },
          }),
        ]),
      ])
    );
  };

  renderAccount = (user) => {
    this.domComponent.append(
      createElement("div", { class: "modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, "Account"),
          createElement("button", { class: "close-modal" }, "×", {
            type: "click",
            event: this.close,
          }),
        ]),
        createElement("div", { class: "modal-body" }, [
          createElement("div", {
            id: "toast-message",
            class: "toast-warning",
          }),
          createElement("h3", {}, `Username: "${user.username}"`),
          createElement(
            "form",
            { id: "update-username-form" },
            [
              createElement("fieldset", {}, [
                createElement("legend", {}, "Update Username"),

                createElement("div", { class: "input-container" }, [
                  createElement("label", { for: "username" }, "Username"),
                  createElement("input", {
                    name: "username",
                    type: "text",
                    required: true,
                    autofocus: true,
                    value: user.username || "",
                  }),
                ]),
                createElement("br"),
                createElement("button", { type: "submit" }, "Submit"),
              ]),
            ],
            {
              type: "submit",
              event: (e) => {
                e.preventDefault();

                const form = e.target;
                const formData = new FormData(form);
                const jsonData = Object.fromEntries(formData.entries());
                jsonData["user_id"] = user.id;

                this.socketConn.updateUsername(jsonData);
              },
            }
          ),
          createElement("br"),
          createElement("button", { class: "btn-red" }, "Logout", {
            type: "click",
            event: (e) => {
              window.go.main.App.RemoveAuthToken(this.socketConn.hostUUID).then(() => {
                console.log("Token removed for", this.socketConn.hostUUID);
              });
              this.open({
                type: "login",
                data: {},
              });
            },
          }),
        ]),
      ])
    );
  };

  renderInvites = (invites, user) => {
    this.domComponent.append(
      createElement("div", { class: "modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, `Space Invites`),
          createElement("button", { class: "close-modal" }, "×", {
            type: "click",
            event: this.close,
          }),
        ]),
        createElement(
          "div",
          { class: "modal-body" },
          invites && invites.length && user
            ? createElement("div", { class: "invites-section" }, [
                ...invites.map((invite) =>
                  createElement("div", { class: "pending-invites-item" }, [
                    createElement("span", {}, invite.name),
                    createElement("div", {}, [
                      createElement(
                        "button",
                        { class: "btn-small accept-invite" },
                        "Accept",
                        {
                          type: "click",
                          event: () =>
                            this.socketConn.acceptInvite({
                              space_user_id: invite.id,
                              user_id: user.id,
                            }),
                        }
                      ),
                      createElement(
                        "button",
                        { class: "btn-small btn-red decline-invite" },
                        "Decline",
                        {
                          type: "click",
                          event: () =>
                            this.socketConn.declineInvite({
                              space_user_id: invite.id,
                              user_id: user.id,
                            }),
                        }
                      ),
                    ]),
                  ])
                ),
              ])
            : "...No Invite Yet"
        ),
      ])
    );
  };

  render = (props) => {
    this.domComponent.innerHTML = "";

    switch (props.type) {
      case "login":
        this.renderLoginForm();
        break;
      case "register":
        this.renderRegisterForm();
        break;
      case "account":
        if (props.data.user) {
          this.renderAccount(props.data.user);
        }
        break;
      case "invites":
        this.renderInvites(props.data.invites, props.data.user);
        break;
      case "space-settings":
        if (props.data.space && props.data.user) {
          const space = props.data.space;
          const user = props.data.user;
          const isAuthor = space.author_id === user.id;

          this.domComponent.append(
            createElement("div", { class: "modal-content" }, [
              createElement("div", { class: "modal-header" }, [
                createElement("h2", {}, `Space Settings: ${space.name}`),
                createElement("button", { class: "close-modal" }, "×", {
                  type: "click",
                  event: this.close,
                }),
              ]),
              createElement(
                "div",
                { class: "modal-body" },
                isAuthor
                  ? this.renderAuthorSettings(space)
                  : this.renderSpaceUserSettings(space, user)
              ),
            ])
          );
        }
        break;
      default:
        console.log("");
    }
  };
}
