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
                    user_public_key: user.public_key || "",
                    user_enc_public_key: user.enc_public_key || "",
                  });
                }
              });
            },
          }),
        ]),
      ]),
    ];
  };

  renderAccount = (user, activeDevices = []) => {
    const importIdentityText = async (importText, passphrase) => {
      if (!importText) {
        platform.alert("No backup data provided");
        return;
      }
      if (!passphrase || passphrase.length < 8) {
        platform.alert("Enter an import passphrase (8+ chars)");
        return;
      }

      const confirmed = await platform.confirm(
        "Import identity and replace the current local identity on this browser?"
      );
      if (!confirmed) return;

      try {
        await identityManager.importEncryptedIdentity(importText, passphrase);
        platform.alert("Identity imported. Reloading...");
        window.location.reload();
      } catch (err) {
        console.error(err);
        platform.alert(err?.message || "Failed to import encrypted backup");
      }
    };
    const formatDateTime = (isoText) => {
      if (!isoText) return "Never";
      const date = new Date(isoText);
      if (Number.isNaN(date.getTime())) return "Unknown";
      return date.toLocaleString();
    };
    const updateLastExportedLabel = () => {
      const exportMetaElem = this.domComponent.querySelector("#last-exported-at");
      if (!exportMetaElem) return;
      exportMetaElem.textContent = formatDateTime(identityManager.getLastExportedAt());
    };
    const renderActiveDevices = () => {
      const listElem = this.domComponent.querySelector("#active-devices-list");
      if (!listElem) return;
      listElem.innerHTML = "";
      if (!Array.isArray(activeDevices) || activeDevices.length === 0) {
        listElem.append(
          createElement(
            "div",
            { style: "font-size: 0.85rem; color: var(--main-gray);" },
            "No active device sessions detected on this host."
          )
        );
        return;
      }
      const nodes = activeDevices.map((device) => {
        const label = `${device.device_name || "Unknown Device"}${device.is_current ? " (This device)" : ""}`;
        const details = `Last seen: ${formatDateTime(device.last_seen || "")}`;
        return createElement("div", { class: "channel-item" }, [
          createElement("div", { style: "font-size: 0.92rem; color: var(--bright-white);" }, label),
          createElement("div", { style: "font-size: 0.8rem; color: var(--main-gray);" }, details),
        ]);
      });
      listElem.append(...nodes);
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
            createElement("h3", {}, "Encrypted Identity Backup"),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--light-red);" },
              "Backups are encrypted with your passphrase. If you lose the passphrase and backup file, your identity cannot be recovered."
            ),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--main-gray);" },
              "Private keys stay non-exportable during normal use. Export creates an encrypted backup file for device transfer."
            ),
            createElement("div", { class: "input-container" }, [
              createElement("label", { for: "identity-export-passphrase" }, "Backup Passphrase"),
              createElement("input", {
                id: "identity-export-passphrase",
                type: "password",
                placeholder: "At least 8 characters",
                autocomplete: "new-password",
              }),
            ]),
            createElement("br"),
            createElement("textarea", {
              id: "identity-export-display",
              readonly: "readonly",
              rows: "6",
              placeholder: "Encrypted backup JSON appears here after generation",
              style: "width: 100%; font-size: 0.78rem; padding: 8px; background: var(--second-dark); color: var(--main-white); border: 1px solid var(--main-gray); border-radius: 4px; resize: vertical; margin-bottom: 8px;",
            }),
            createElement("button", { class: "btn", id: "generate-identity-backup-btn" }, "Generate Encrypted Backup", {
              type: "click",
              event: async () => {
                const passphraseElem = this.domComponent.querySelector("#identity-export-passphrase");
                const exportElem = this.domComponent.querySelector("#identity-export-display");
                const passphrase = (passphraseElem?.value || "").trim();
                if (passphrase.length < 8) {
                  platform.alert("Enter a backup passphrase (8+ chars)");
                  return;
                }
                try {
                  const backupText = await identityManager.exportEncryptedIdentity(passphrase);
                  if (exportElem) {
                    exportElem.value = backupText;
                  }
                } catch (err) {
                  console.error(err);
                  platform.alert(err?.message || "Failed to generate encrypted backup");
                }
              },
            }),
            createElement("button", { class: "btn", id: "copy-identity-btn", style: "margin-left: 8px;" }, "Copy Backup JSON", {
              type: "click",
              event: async () => {
                const exportElem = this.domComponent.querySelector("#identity-export-display");
                const text = (exportElem?.value || "").trim();
                if (!text) {
                  platform.alert("Generate backup first");
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
                  identityManager.markIdentityExportedNow();
                  updateLastExportedLabel();
                  platform.alert("Encrypted backup copied");
                } catch (_err) {
                  platform.alert("Failed to copy backup");
                }
              },
            }),
            createElement("button", { class: "btn", id: "download-identity-btn", style: "margin-left: 8px;" }, "Download Backup File", {
              type: "click",
              event: async () => {
                const passphraseElem = this.domComponent.querySelector("#identity-export-passphrase");
                const exportElem = this.domComponent.querySelector("#identity-export-display");
                const passphrase = (passphraseElem?.value || "").trim();
                if (passphrase.length < 8) {
                  platform.alert("Enter your backup passphrase first");
                  return;
                }
                try {
                  const exportText = (exportElem?.value || "").trim() || await identityManager.exportEncryptedIdentity(passphrase);
                  if (exportElem && !exportElem.value) {
                    exportElem.value = exportText;
                  }
                  const blob = new Blob([exportText], { type: "application/json" });
                  const url = URL.createObjectURL(blob);
                  const link = document.createElement("a");
                  link.href = url;
                  link.download = "parch-identity-backup.enc.json";
                  document.body.appendChild(link);
                  link.click();
                  link.remove();
                  URL.revokeObjectURL(url);
                  identityManager.markIdentityExportedNow();
                  updateLastExportedLabel();
                } catch (err) {
                  console.error(err);
                  platform.alert(err?.message || "Failed to generate backup file");
                }
              },
            }),
            createElement(
              "p",
              { style: "display: block; margin: 8px 0 6px; font-size: 0.82rem; color: var(--main-gray);" },
              ["Last exported: ", createElement("span", { id: "last-exported-at" }, "Never")]
            ),
            createElement("br"),
            createElement("div", { class: "input-container" }, [
              createElement("label", { for: "identity-import-passphrase" }, "Import Passphrase"),
              createElement("input", {
                id: "identity-import-passphrase",
                type: "password",
                placeholder: "Passphrase used to encrypt the backup",
                autocomplete: "off",
              }),
            ]),
            createElement("br"),
            createElement("textarea", {
              id: "identity-import-input",
              rows: "6",
              placeholder: "Paste encrypted backup JSON here to import",
              style: "width: 100%; font-size: 0.78rem; padding: 8px; background: var(--second-dark); color: var(--a-blue); border: 1px solid var(--main-gray); border-radius: 4px; resize: vertical; margin-bottom: 8px;",
            }),
            createElement("button", { class: "btn", id: "import-identity-btn" }, "Import Encrypted Backup", {
              type: "click",
              event: async () => {
                const importElem = this.domComponent.querySelector("#identity-import-input");
                const importPassphraseElem = this.domComponent.querySelector("#identity-import-passphrase");
                const importText = (importElem?.value || "").trim();
                const passphrase = (importPassphraseElem?.value || "").trim();

                if (!importText) {
                  platform.alert("Paste encrypted backup JSON to import");
                  return;
                }
                await importIdentityText(importText, passphrase);
              },
            }),
            createElement("button", { class: "btn", id: "import-identity-file-btn", style: "margin-left: 8px;" }, "Import Backup File", {
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
                  const importElem = this.domComponent.querySelector("#identity-import-input");
                  if (importElem) {
                    importElem.value = text;
                  }
                } catch (_err) {
                  platform.alert("Failed to read backup file");
                }
              },
            }),
          ]),
          createElement("div", { class: "settings-section" }, [
            createElement("h3", {}, "Active Devices"),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--main-gray);" },
              "Shows active sessions for this identity on this host."
            ),
            createElement("div", { id: "active-devices-list", class: "channels-list" }),
          ]),
          createElement("div", { class: "settings-section" }, [
            createElement("h3", {}, "Local Fallback Username"),
            createElement(
              "p",
              { style: "display: block; margin-bottom: 8px; font-size: 0.85rem; color: var(--main-gray);" },
              "Used as the default username sent during auth when joining a host for the first time."
            ),
            createElement("div", { class: "input-container" }, [
              createElement("label", { for: "local-identity-username" }, "Local Username"),
              createElement("input", {
                id: "local-identity-username",
                name: "localIdentityUsername",
                type: "text",
                value: "",
                placeholder: "Set local fallback username",
              }),
            ]),
            createElement("br"),
            createElement("button", { class: "btn" }, "Save Local Username", {
              type: "click",
              event: () => {
                const localUsernameElem = this.domComponent.querySelector("#local-identity-username");
                const localUsername = (localUsernameElem?.value || "").trim();
                identityManager.setIdentityUsername(localUsername);
                platform.alert("Local fallback username saved");
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
                    id: "username",
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
                jsonData["user_public_key"] = user.public_key || "";
                jsonData["user_enc_public_key"] = user.enc_public_key || "";
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
        const localUsernameElem = this.domComponent.querySelector("#local-identity-username");
        if (publicKeyElem) {
          publicKeyElem.value = identity.publicKey || "";
        }
        if (localUsernameElem) {
          localUsernameElem.value = identity.username || "";
        }
      })
      .catch(() => {
        const publicKeyElem = this.domComponent.querySelector("#public-key-display");
        const exportElem = this.domComponent.querySelector("#identity-export-display");
        const localUsernameElem = this.domComponent.querySelector("#local-identity-username");
        if (publicKeyElem) {
          publicKeyElem.value = "";
        }
        if (localUsernameElem) {
          localUsernameElem.value = "";
        }
        if (exportElem) {
          exportElem.value = "";
        }
      });
    updateLastExportedLabel();
    renderActiveDevices();
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
                            user_public_key: user.public_key || "",
                            user_enc_public_key: user.enc_public_key || "",
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
                              user_public_key: user.public_key || "",
                              user_enc_public_key: user.enc_public_key || "",
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
          this.renderAccount(props.data.user, props.data.active_devices || []);
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
