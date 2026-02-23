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
                        this.socketConn.deleteChannel({
                          uuid: channel.uuid,
                          space_uuid: space.uuid,
                        });
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
    const formatDateTime = (isoText) => {
      if (!isoText) return "Never";
      const date = new Date(isoText);
      if (Number.isNaN(date.getTime())) return "Unknown";
      return date.toLocaleString();
    };

    const shortKey = (key) => {
      if (!key || typeof key !== "string") return "Unknown";
      if (key.length <= 20) return key;
      return `${key.slice(0, 10)}...${key.slice(-8)}`;
    };

    const copyText = async (text, sourceElem) => {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
        return;
      }
      if (!sourceElem) {
        throw new Error("No copy source");
      }
      sourceElem.focus();
      sourceElem.select();
      document.execCommand("copy");
    };

    const generateBackupText = async () => {
      const passphraseElem = this.domComponent.querySelector("#identity-export-passphrase");
      const exportElem = this.domComponent.querySelector("#identity-export-display");
      const passphrase = (passphraseElem?.value || "").trim();
      if (passphrase.length < 8) {
        throw new Error("Enter a backup passphrase (8+ chars)");
      }
      const backupText = await identityManager.exportEncryptedIdentity(passphrase);
      if (exportElem) {
        exportElem.value = backupText;
      }
      return backupText;
    };

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
            { class: "account-device-empty" },
            "No active device sessions detected on this host."
          )
        );
        return;
      }
      const nodes = activeDevices.map((device) => {
        const label = `${device.device_name || "Unknown Device"}${device.is_current ? " (This device)" : ""}`;
        const details = `Last seen: ${formatDateTime(device.last_seen || "")}`;
        return createElement("div", { class: "account-device-item" }, [
          createElement("div", { class: "account-device-name" }, label),
          createElement("div", { class: "account-device-meta" }, details),
        ]);
      });
      listElem.append(...nodes);
    };

    this.domComponent.append(
      createElement("div", { class: "modal-content account-modal-content" }, [
        createElement("div", { class: "modal-header" }, [
          createElement("h2", {}, "Account"),
          createElement("button", { class: "close-modal" }, "×", {
            type: "click",
            event: this.close,
          }),
        ]),
        createElement("div", { class: "modal-body account-modal-body" }, [
          createElement("div", { class: "account-overview" }, [
            createElement("div", { class: "account-overview-row" }, [
              createElement("div", { class: "account-overview-title" }, user.username || "unknown"),
              createElement("div", { class: "account-overview-subtitle" }, `Host User ID: ${user.id || "unknown"}`),
            ]),
            createElement("div", { class: "account-overview-meta" }, [
              createElement("span", { class: "account-meta-label" }, "Last Exported"),
              createElement("span", { id: "last-exported-at", class: "account-meta-value" }, "Never"),
            ]),
          ]),
          createElement("div", { class: "account-grid" }, [
            createElement("div", { class: "settings-section account-card" }, [
              createElement("div", { class: "account-card-title" }, "Public Key"),
              createElement(
                "p",
                { class: "account-help" },
                "Share this public key with space admins so they can invite this identity."
              ),
              createElement("textarea", {
                id: "public-key-display",
                class: "account-textarea account-textarea--compact",
                readonly: "readonly",
                rows: "4",
              }),
              createElement("div", { class: "account-actions" }, [
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
                      await copyText(publicKey, publicKeyElem);
                      platform.alert("Public key copied");
                    } catch (_err) {
                      platform.alert("Failed to copy public key");
                    }
                  },
                }),
              ]),
              createElement(
                "p",
                { id: "account-key-fingerprint", class: "account-inline-mono" },
                `Fingerprint: ${shortKey(user.public_key || "")}`
              ),
            ]),
            createElement("div", { class: "settings-section account-card" }, [
              createElement("div", { class: "account-card-title" }, "Active Devices"),
              createElement(
                "p",
                { class: "account-help" },
                "Sessions currently active for this identity on this host."
              ),
              createElement("div", { id: "active-devices-list", class: "account-device-list" }),
            ]),
          ]),
          createElement("div", { class: "account-grid" }, [
            createElement("div", { class: "settings-section account-card" }, [
              createElement("div", { class: "account-card-title" }, "Create Encrypted Backup"),
              createElement(
                "p",
                { class: "account-warning" },
                "If you lose both backup file and passphrase, this identity cannot be recovered."
              ),
              createElement("div", { class: "input-container account-input-wrap" }, [
                createElement("label", { for: "identity-export-passphrase" }, "Backup Passphrase"),
                createElement("input", {
                  id: "identity-export-passphrase",
                  class: "account-input",
                  type: "password",
                  placeholder: "At least 8 characters",
                  autocomplete: "new-password",
                }),
              ]),
              createElement("textarea", {
                id: "identity-export-display",
                class: "account-textarea",
                readonly: "readonly",
                rows: "6",
                placeholder: "Encrypted backup JSON appears here after generation",
              }),
              createElement("div", { class: "account-actions" }, [
                createElement("button", { class: "btn", id: "generate-identity-backup-btn" }, "Generate", {
                  type: "click",
                  event: async () => {
                    try {
                      await generateBackupText();
                    } catch (err) {
                      platform.alert(err?.message || "Failed to generate encrypted backup");
                    }
                  },
                }),
                createElement("button", { class: "btn", id: "copy-identity-btn" }, "Copy Backup", {
                  type: "click",
                  event: async () => {
                    const exportElem = this.domComponent.querySelector("#identity-export-display");
                    const text = (exportElem?.value || "").trim();
                    if (!text) {
                      platform.alert("Generate backup first");
                      return;
                    }
                    try {
                      await copyText(text, exportElem);
                      identityManager.markIdentityExportedNow();
                      updateLastExportedLabel();
                      platform.alert("Encrypted backup copied");
                    } catch (_err) {
                      platform.alert("Failed to copy backup");
                    }
                  },
                }),
                createElement("button", { class: "btn", id: "download-identity-btn" }, "Download File", {
                  type: "click",
                  event: async () => {
                    try {
                      const exportElem = this.domComponent.querySelector("#identity-export-display");
                      const exportText = (exportElem?.value || "").trim() || await generateBackupText();
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
                      platform.alert(err?.message || "Failed to generate backup file");
                    }
                  },
                }),
              ]),
            ]),
            createElement("div", { class: "settings-section account-card" }, [
              createElement("div", { class: "account-card-title" }, "Restore Encrypted Backup"),
              createElement(
                "p",
                { class: "account-help" },
                "Importing will replace the current local identity on this browser."
              ),
              createElement("div", { class: "input-container account-input-wrap" }, [
                createElement("label", { for: "identity-import-passphrase" }, "Import Passphrase"),
                createElement("input", {
                  id: "identity-import-passphrase",
                  class: "account-input",
                  type: "password",
                  placeholder: "Passphrase used for backup",
                  autocomplete: "off",
                }),
              ]),
              createElement("textarea", {
                id: "identity-import-input",
                class: "account-textarea",
                rows: "6",
                placeholder: "Paste encrypted backup JSON here to import",
              }),
              createElement("div", { class: "account-actions" }, [
                createElement("button", { class: "btn", id: "import-identity-btn" }, "Import Backup", {
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
                createElement("button", { class: "btn", id: "import-identity-file-btn" }, "Load File", {
                  type: "click",
                  event: () => {
                    const fileInput = this.domComponent.querySelector("#identity-import-file");
                    if (fileInput) {
                      fileInput.value = "";
                      fileInput.click();
                    }
                  },
                }),
              ]),
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
          ]),
          createElement("div", { class: "account-grid" }, [
            createElement("div", { class: "settings-section account-card" }, [
              createElement("div", { class: "account-card-title" }, "Local Fallback Username"),
              createElement(
                "p",
                { class: "account-help" },
                "Used as the default username sent during auth when this browser joins a host."
              ),
              createElement("div", { class: "input-container account-input-wrap" }, [
                createElement("label", { for: "local-identity-username" }, "Local Username"),
                createElement("input", {
                  id: "local-identity-username",
                  name: "localIdentityUsername",
                  class: "account-input",
                  type: "text",
                  value: "",
                  placeholder: "Set local fallback username",
                }),
              ]),
              createElement("div", { class: "account-actions" }, [
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
            ]),
            createElement("div", { class: "settings-section account-card" }, [
              createElement("div", { class: "account-card-title" }, "Host Profile Username"),
              createElement(
                "p",
                { class: "account-help" },
                "This updates your username for this host only."
              ),
              createElement("form", { id: "update-username-form", class: "account-form" }, [
                createElement("div", { class: "input-container account-input-wrap" }, [
                  createElement("label", { for: "username" }, "Host Username"),
                  createElement("input", {
                    id: "username",
                    name: "username",
                    class: "account-input",
                    type: "text",
                    required: true,
                    value: user.username || "",
                  }),
                ]),
                createElement("div", { class: "account-actions" }, [
                  createElement("button", { type: "submit" }, "Update Username"),
                ]),
              ], {
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
              }),
            ]),
          ]),
          createElement("div", { class: "settings-section account-card account-danger-zone" }, [
            createElement("div", { class: "account-card-title account-card-title--danger" }, "Danger Zone"),
            createElement(
              "p",
              { class: "account-help" },
              "Reset identity creates a new keypair and account identity for this browser."
            ),
            createElement("div", { class: "account-actions" }, [
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
          ]),
        ]),
      ])
    );

    identityManager
      .getOrCreateIdentity()
      .then((identity) => {
        const publicKeyElem = this.domComponent.querySelector("#public-key-display");
        const localUsernameElem = this.domComponent.querySelector("#local-identity-username");
        const keyFingerprint = this.domComponent.querySelector("#account-key-fingerprint");
        if (publicKeyElem) {
          publicKeyElem.value = identity.publicKey || "";
        }
        if (localUsernameElem) {
          localUsernameElem.value = identity.username || "";
        }
        if (keyFingerprint) {
          keyFingerprint.textContent = `Fingerprint: ${shortKey(identity.publicKey || "")}`;
        }
      })
      .catch(() => {
        const publicKeyElem = this.domComponent.querySelector("#public-key-display");
        const localUsernameElem = this.domComponent.querySelector("#local-identity-username");
        const exportElem = this.domComponent.querySelector("#identity-export-display");
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
