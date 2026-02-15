import createElement from "./createElement.js";
import platform from "../platform/index.js";
import identityManager from "../lib/identityManager.js";

export default class DashModal {
  constructor(app, socketConn) {
    this.app = app;
    this.socketConn = socketConn;
    this.domComponent = createElement("div", { class: "modal" });
    this.promptResolver = null;
  }

  open = (props) => {
    this.domComponent.style.display = "block";
    return this.render(props);
  };

  close = () => {
    this.domComponent.style.display = "none";
  };

  renderPrompt = (message) => {
    return new Promise((resolve) => {
      this.promptResolver = resolve;

      this.domComponent.append(
        createElement("div", { class: "modal-content" }, [
          createElement("div", { class: "modal-header" }, [
            createElement("h2", {}, message || "Enter a value"),
          ]),
          createElement("div", { class: "modal-body" }, [
            createElement("input", {
              id: "prompt-input",
              type: "text",
              style: "width: 100%; margin-bottom: 10px;",
            }),
            createElement("div", { style: "text-align: right;" }, [
              createElement("button", { class: "btn" }, "OK", {
                type: "click",
                event: () => {
                  const val = this.domComponent.querySelector("#prompt-input").value;
                  this.close();
                  this.promptResolver(val);
                },
              }),
              createElement(
                "button",
                { class: "btn btn-red", style: "margin-left: 10px;" },
                "Cancel",
                {
                  type: "click",
                  event: () => {
                    this.close();
                    this.promptResolver(null);
                  },
                }
              ),
            ]),
          ]),
        ])
      );
    });
  };

  renderAuthorSettings = (space) => {
    return [
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Space Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn" }, "Invite User", {
            type: "click",
            event: () => {
              this.render({
                type: "prompt",
                data: { message: "Enter recipient public key" },
              }).then((value) => {
                if (value) {
                  this.socketConn.inviteUser({
                    public_key: value.trim(),
                    space_uuid: space.uuid,
                  });
                }
              });
            },
          }),
          createElement("button", { class: "btn btn-red" }, "Delete Space", {
            type: "click",
            event: () => {
              platform.confirm("Are you sure you want to delete this space?").then((confirmed) => {
                if (confirmed) {
                  this.socketConn.deleteSpace({ uuid: space.uuid });
                }
              });
            },
          }),
        ]),
      ]),
      createElement("div", { class: "settings-section" }, [
        createElement("h3", {}, "Channel Management"),
        createElement("div", { class: "settings-actions" }, [
          createElement("button", { class: "btn" }, "+ Create Channel", {
            type: "click",
            event: () => {
              this.render({
                type: "prompt",
                data: { message: "Please enter Channel name" },
              }).then((value) => {
                if (value) {
                  this.socketConn.createChannel({
                    name: value.trim(),
                    space_uuid: space.uuid,
                  });
                }
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
                    platform.confirm("Are you sure you want to delete this channel?").then((confirmed) => {
                      if (confirmed) {
                        this.socketConn.deleteChannel({ uuid: channel.uuid });
                      }
                    });
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
              platform.confirm("Are you sure you want to leave this space?").then((confirmed) => {
                if (confirmed) {
                  this.socketConn.leaveSpace({
                    space_uuid: space.uuid,
                    user_id: user.id,
                  });
                }
              });
            },
          }),
        ]),
      ]),
    ];
  };

  renderAccount = (user) => {
    const importIdentityText = async (importText) => {
      if (!importText) {
        platform.alert("No identity data provided");
        return;
      }

      const confirmed = await platform.confirm(
        "Import identity and replace the current local identity on this browser?"
      );
      if (!confirmed) return;

      try {
        await identityManager.importIdentity(importText);
        platform.alert("Identity imported. Reloading...");
        window.location.reload();
      } catch (err) {
        console.error(err);
        platform.alert(err?.message || "Failed to import identity");
      }
    };

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
          createElement("h3", {}, `Username: "${user.username}"`),
          createElement("div", { class: "settings-section" }, [
            createElement("h3", {}, "Public Key (Share for Invites)"),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--main-gray);" },
              "Share this public key with space admins so they can invite you."
            ),
            createElement("textarea", {
              id: "public-key-display",
              readonly: "readonly",
              rows: "4",
              style: "width: 100%; font-size: 0.8rem; padding: 8px; background: var(--second-dark); color: var(--a-blue); border: 1px solid var(--main-gray); border-radius: 4px; resize: vertical;",
            }),
            createElement("br"),
            createElement("button", { class: "btn", id: "copy-public-key-btn" }, "Copy Public Key", {
              type: "click",
              event: async () => {
                const publicKeyElem = this.domComponent.querySelector("#public-key-display");
                const publicKey = (publicKeyElem?.value || "").trim();
                if (!publicKey) {
                  platform.alert("Public key not available yet");
                  return;
                }
                try {
                  if (navigator.clipboard?.writeText) {
                    await navigator.clipboard.writeText(publicKey);
                  } else {
                    publicKeyElem.focus();
                    publicKeyElem.select();
                    document.execCommand("copy");
                  }
                  platform.alert("Public key copied");
                } catch (_err) {
                  platform.alert("Failed to copy public key");
                }
              },
            }),
          ]),
          createElement("div", { class: "settings-section" }, [
            createElement("h3", {}, "Identity Transfer (Private Key)"),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--light-red);" },
              "Anyone with your identity JSON can access your account. Share it only with yourself."
            ),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--main-gray);" },
              "Export on this browser, then import on another browser/device to keep the same identity."
            ),
            createElement("textarea", {
              id: "identity-export-display",
              readonly: "readonly",
              rows: "6",
              style: "width: 100%; font-size: 0.78rem; padding: 8px; background: var(--second-dark); color: var(--main-white); border: 1px solid var(--main-gray); border-radius: 4px; resize: vertical; margin-bottom: 8px;",
            }),
            createElement("button", { class: "btn", id: "copy-identity-btn" }, "Copy Identity JSON", {
              type: "click",
              event: async () => {
                const exportElem = this.domComponent.querySelector("#identity-export-display");
                const text = (exportElem?.value || "").trim();
                if (!text) {
                  platform.alert("Identity export is not ready");
                  return;
                }
                try {
                  if (navigator.clipboard?.writeText) {
                    await navigator.clipboard.writeText(text);
                  } else {
                    exportElem.focus();
                    exportElem.select();
                    document.execCommand("copy");
                  }
                  platform.alert("Identity JSON copied");
                } catch (_err) {
                  platform.alert("Failed to copy identity JSON");
                }
              },
            }),
            createElement("button", { class: "btn", id: "download-identity-btn", style: "margin-left: 8px;" }, "Download Identity File", {
              type: "click",
              event: async () => {
                try {
                  const exportText = await identityManager.exportIdentity();
                  const blob = new Blob([exportText], { type: "application/json" });
                  const url = URL.createObjectURL(blob);
                  const link = document.createElement("a");
                  link.href = url;
                  link.download = "parch-identity.json";
                  document.body.appendChild(link);
                  link.click();
                  link.remove();
                  URL.revokeObjectURL(url);
                } catch (_err) {
                  platform.alert("Failed to generate identity file");
                }
              },
            }),
            createElement("br"),
            createElement("textarea", {
              id: "identity-import-input",
              rows: "6",
              placeholder: "Paste identity JSON here to import",
              style: "width: 100%; font-size: 0.78rem; padding: 8px; background: var(--second-dark); color: var(--a-blue); border: 1px solid var(--main-gray); border-radius: 4px; resize: vertical; margin-bottom: 8px;",
            }),
            createElement("button", { class: "btn", id: "import-identity-btn" }, "Import Identity JSON", {
              type: "click",
              event: async () => {
                const importElem = this.domComponent.querySelector("#identity-import-input");
                const importText = (importElem?.value || "").trim();

                if (!importText) {
                  platform.alert("Paste identity JSON to import");
                  return;
                }
                await importIdentityText(importText);
              },
            }),
            createElement("button", { class: "btn", id: "import-identity-file-btn", style: "margin-left: 8px;" }, "Import Identity File", {
              type: "click",
              event: () => {
                const fileInput = this.domComponent.querySelector("#identity-import-file");
                if (fileInput) {
                  fileInput.value = "";
                  fileInput.click();
                }
              },
            }),
            createElement("input", {
              id: "identity-import-file",
              type: "file",
              accept: "application/json,.json",
              style: "display: none;",
            }, null, {
              type: "change",
              event: async (event) => {
                const file = event?.target?.files?.[0];
                if (!file) return;
                try {
                  const text = await file.text();
                  await importIdentityText(text);
                } catch (_err) {
                  platform.alert("Failed to read identity file");
                }
              },
            }),
          ]),
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
                const formData = new FormData(e.target);
                const jsonData = Object.fromEntries(formData.entries());
                jsonData["user_id"] = user.id;
                this.socketConn.updateUsername(jsonData);
              },
            }
          ),
          createElement("br"),
          createElement("button", { class: "btn-red" }, "Reset Identity", {
            type: "click",
            event: () => {
              platform.confirm("Reset local identity key? This creates a new account identity.").then((confirmed) => {
                if (!confirmed) return;
                identityManager.clearIdentity();
                window.location.reload();
              });
            },
          }),
        ]),
      ])
    );

    identityManager
      .getOrCreateIdentity()
      .then((identity) => {
        const publicKeyElem = this.domComponent.querySelector("#public-key-display");
        const exportElem = this.domComponent.querySelector("#identity-export-display");
        if (publicKeyElem) {
          publicKeyElem.value = identity.publicKey || "";
        }
        if (exportElem) {
          identityManager
            .exportIdentity()
            .then((exported) => {
              exportElem.value = exported;
            })
            .catch(() => {
              exportElem.value = "";
            });
        }
      })
      .catch(() => {
        const publicKeyElem = this.domComponent.querySelector("#public-key-display");
        const exportElem = this.domComponent.querySelector("#identity-export-display");
        if (publicKeyElem) {
          publicKeyElem.value = "";
        }
        if (exportElem) {
          exportElem.value = "";
        }
      });
  };

  renderInvites = (invites, user) => {
    this.domComponent.append(
      createElement("div", { class: "modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, "Space Invites"),
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
                      createElement("button", { class: "btn-small accept-invite" }, "Accept", {
                        type: "click",
                        event: () =>
                          this.socketConn.acceptInvite({
                            space_user_id: invite.id,
                            user_id: user.id,
                          }),
                      }),
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
      case "prompt":
        if (props.data?.message) {
          return this.renderPrompt(props.data.message);
        }
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
                isAuthor ? this.renderAuthorSettings(space) : this.renderSpaceUserSettings(space, user)
              ),
            ])
          );
        }
        break;
      default:
        break;
    }
  };
}
