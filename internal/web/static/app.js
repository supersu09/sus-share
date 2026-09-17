(function () {
  "use strict";

  const state = { items: [], files: [] };
  const authView = document.getElementById("auth-view");
  const boardView = document.getElementById("board-view");
  const itemsList = document.getElementById("items-list");
  const emptyState = document.getElementById("empty-state");
  const refreshButton = document.getElementById("refresh-button");
  const refreshState = document.getElementById("refresh-state");
  const toast = document.getElementById("toast");
  const pageBase = new URL(".", window.location.href);
  const loginPath = makeURL("api/v1/auth/login");
  const composer = document.getElementById("composer");
  const composerResize = document.getElementById("composer-resize");
  const COMPOSER_DEFAULT_HEIGHT = 200;
  const COMPOSER_MIN_HEIGHT = 176;
  const COMPOSER_VIEWPORT_RESERVE = 112;
  const COMPOSER_KEYBOARD_STEP = 16;

  const errorMessages = new Map([
    ["authentication required", "请先登录"],
    ["authentication failed", "密码错误"],
    ["invalid JSON body", "请求格式错误"],
    ["expected JSON", "请求格式错误"],
    ["limit exceeded", "超出容量限制"],
    ["internal error", "服务器错误"],
    ["could not persist settings", "设置保存失败"],
    ["could not persist password", "密码保存失败"]
  ]);

  let toastTimer;
  let composerHeight = COMPOSER_DEFAULT_HEIGHT;
  let activePointerId = null;
  let dragStartY = 0;
  let dragStartHeight = composerHeight;

  function makeURL(path) {
    return new URL(path.replace(/^\/+/, ""), pageBase).href;
  }

  function apiURL(...parts) {
    return makeURL(["api/v1", ...parts].join("/"));
  }

  function show(node, visible) {
    node.hidden = !visible;
  }

  function composerMaxHeight() {
    return Math.max(COMPOSER_MIN_HEIGHT, window.innerHeight - COMPOSER_VIEWPORT_RESERVE);
  }

  function updateComposerResizeAttributes() {
    const maxHeight = composerMaxHeight();
    composerResize.setAttribute("aria-valuemin", String(COMPOSER_MIN_HEIGHT));
    composerResize.setAttribute("aria-valuemax", String(Math.round(maxHeight)));
    composerResize.setAttribute("aria-valuenow", String(Math.round(composerHeight)));
    composerResize.setAttribute("aria-valuetext", `${Math.round(composerHeight)} 像素`);
  }

  function setComposerHeight(height) {
    composerHeight = Math.round(Math.min(composerMaxHeight(), Math.max(COMPOSER_MIN_HEIGHT, height)));
    composer.style.setProperty("--composer-height", `${composerHeight}px`);
    updateComposerResizeAttributes();
  }

  function stopComposerDrag(pointerId) {
    if (pointerId !== activePointerId) return;
    if (composerResize.hasPointerCapture(pointerId)) composerResize.releasePointerCapture(pointerId);
    activePointerId = null;
    composerResize.classList.remove("dragging");
  }

  composerResize.addEventListener("pointerdown", event => {
    if (activePointerId !== null) return;
    if (event.pointerType === "mouse" && event.button !== 0) return;
    event.preventDefault();
    activePointerId = event.pointerId;
    dragStartY = event.clientY;
    dragStartHeight = composerHeight;
    composerResize.setPointerCapture(event.pointerId);
    composerResize.classList.add("dragging");
  });

  composerResize.addEventListener("pointermove", event => {
    if (event.pointerId !== activePointerId) return;
    setComposerHeight(dragStartHeight + dragStartY - event.clientY);
  });

  ["pointerup", "pointercancel", "lostpointercapture"].forEach(type => {
    composerResize.addEventListener(type, event => stopComposerDrag(event.pointerId));
  });

  composerResize.addEventListener("keydown", event => {
    if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
    event.preventDefault();
    const direction = event.key === "ArrowUp" ? 1 : -1;
    setComposerHeight(composerHeight + direction * COMPOSER_KEYBOARD_STEP);
  });

  window.addEventListener("resize", () => setComposerHeight(composerHeight));
  setComposerHeight(COMPOSER_DEFAULT_HEIGHT);

  function showToast(message) {
    toast.textContent = message;
    toast.classList.add("visible");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toast.classList.remove("visible"), 2400);
  }

  function userError(error) {
    const message = error && error.message ? error.message : "";
    return errorMessages.get(message) || "操作失败";
  }

  async function request(path, options = {}) {
    const headers = new Headers(options.headers);
    if (options.body && !(options.body instanceof FormData)) headers.set("Content-Type", "application/json");
    if (options.mutate) headers.set("X-SusShare-Request", "1");
    const response = await fetch(path, { ...options, headers });
    if (response.status === 401 && path !== loginPath) {
      show(boardView, false);
      show(authView, true);
      throw new Error("请先登录");
    }
    return response;
  }

  async function json(path, options = {}) {
    const response = await request(path, options);
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(payload.error || "操作失败");
    return payload;
  }

  function formatSize(size) {
    if (size < 1024) return `${size} B`;
    if (size < 1048576) return `${(size / 1024).toFixed(0)} KB`;
    if (size < 1073741824) return `${(size / 1048576).toFixed(0)} MB`;
    return `${(size / 1073741824).toFixed(1)} GB`;
  }

  function formatTime(value) {
    const created = new Date(value);
    const seconds = Math.max(0, Math.round((Date.now() - created.getTime()) / 1000));
    if (seconds < 60) return "刚刚";
    if (seconds < 3600) return `${Math.floor(seconds / 60)} 分钟前`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)} 小时前`;
    return created.toLocaleString("zh-CN");
  }

  function actionButton(label, className, action) {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = label;
    button.className = className;
    button.addEventListener("click", action);
    return button;
  }

  function addItems(newItems) {
    const validItems = newItems.filter(Boolean);
    if (!validItems.length) return;
    state.items = [...validItems.slice().reverse(), ...state.items];
    renderItems();
  }

  async function loadItems(showLoading) {
    if (showLoading) {
      refreshButton.disabled = true;
      refreshState.textContent = "刷新中";
    }
    try {
      const payload = await json(apiURL("items"));
      state.items = payload.items || [];
      renderItems();
      refreshState.textContent = "";
    } catch (error) {
      if (error.message !== "请先登录") {
        refreshState.textContent = "刷新失败";
        showToast(userError(error));
      }
    } finally {
      refreshButton.disabled = false;
    }
  }

  function renderItems() {
    itemsList.replaceChildren();
    show(emptyState, state.items.length === 0);
    state.items.forEach(item => {
      const article = document.createElement("article");
      article.className = "item-card";

      const top = document.createElement("div");
      top.className = "flex items-start gap-3";
      const title = document.createElement("h3");
      title.className = "item-title";
      title.textContent = item.title;
      const size = document.createElement("span");
      size.className = "item-size";
      size.textContent = formatSize(item.size);
      top.append(title, size);

      const meta = document.createElement("p");
      meta.className = "item-meta";
      meta.textContent = formatTime(item.created_at);

      const actions = document.createElement("div");
      actions.className = "flex flex-wrap justify-end gap-2";
      if (item.kind === "text") {
        actions.append(
          actionButton("复制", "secondary-button", async () => {
            try {
              const response = await request(apiURL("items", item.id, "content"));
              if (!response.ok) throw new Error("复制失败");
              await navigator.clipboard.writeText(await response.text());
              showToast("已复制");
            } catch (error) {
              showToast(userError(error));
            }
          }),
          actionButton("查看", "secondary-button", () => showPreview(item))
        );
      } else {
        actions.append(
          actionButton(item.preview === "download" ? "下载" : "预览", "secondary-button", () => {
            if (item.preview === "download") window.location.href = apiURL("items", item.id, "content");
            else showPreview(item);
          })
        );
        if (item.preview !== "download") {
          actions.append(actionButton("下载", "secondary-button", () => {
            window.location.href = apiURL("items", item.id, "content");
          }));
        }
      }
      actions.append(actionButton("删除", "danger-button", async () => {
        if (!window.confirm("确认删除这条内容？")) return;
        try {
          await json(apiURL("items", item.id), { method: "DELETE", mutate: true });
          state.items = state.items.filter(current => current.id !== item.id);
          renderItems();
          showToast("已删除");
        } catch (error) {
          showToast(userError(error));
        }
      }));

      article.append(top, meta, actions);
      itemsList.append(article);
    });
  }

  async function showPreview(item) {
    const modal = document.getElementById("preview-modal");
    const body = document.getElementById("preview-body");
    document.getElementById("preview-title").textContent = item.title;
    body.replaceChildren();

    const showDownloadFallback = () => {
      body.replaceChildren();
      const paragraph = document.createElement("p");
      paragraph.className = "text-sm text-neutral-600";
      paragraph.textContent = "当前浏览器不支持预览该文件。";
      const download = actionButton("下载文件", "secondary-button", () => {
        window.location.href = apiURL("items", item.id, "content");
      });
      body.append(paragraph, download);
    };

    try {
      if (item.kind === "text" || item.preview === "text") {
        const response = await request(apiURL("items", item.id, "content"));
        if (!response.ok) throw new Error("读取失败");
        const pre = document.createElement("pre");
        pre.className = "preview-text";
        pre.textContent = await response.text();
        body.append(pre);
      } else if (item.preview === "inline-image") {
        const image = document.createElement("img");
        image.className = "preview-media";
        image.src = apiURL("items", item.id, "preview");
        image.alt = item.title;
        image.addEventListener("error", showDownloadFallback);
        body.append(image);
      } else if (item.preview === "inline-video") {
        const video = document.createElement("video");
        video.className = "preview-media";
        video.src = apiURL("items", item.id, "preview");
        video.controls = true;
        video.addEventListener("error", showDownloadFallback);
        body.append(video);
      } else if (item.preview === "inline-audio") {
        const audio = document.createElement("audio");
        audio.className = "w-full";
        audio.src = apiURL("items", item.id, "preview");
        audio.controls = true;
        audio.addEventListener("error", showDownloadFallback);
        body.append(audio);
      } else if (item.preview === "inline-pdf") {
        const frame = document.createElement("iframe");
        frame.className = "preview-frame";
        frame.src = apiURL("items", item.id, "preview");
        frame.title = item.title;
        body.append(frame, actionButton("下载文件", "secondary-button", () => {
          window.location.href = apiURL("items", item.id, "content");
        }));
      } else {
        showDownloadFallback();
      }
      modal.showModal();
    } catch (error) {
      showToast(userError(error));
    }
  }

  document.getElementById("preview-close").addEventListener("click", () => document.getElementById("preview-modal").close());

  document.getElementById("login-form").addEventListener("submit", async event => {
    event.preventDefault();
    const error = document.getElementById("login-error");
    const button = document.getElementById("login-button");
    button.disabled = true;
    try {
      await json(loginPath, {
        method: "POST",
        body: JSON.stringify({ password: document.getElementById("login-password").value }),
        mutate: true
      });
      error.hidden = true;
      document.getElementById("login-password").value = "";
      show(authView, false);
      show(boardView, true);
      await loadItems(false);
    } catch (loginError) {
      error.textContent = loginError.message === "请先登录" ? "密码错误" : userError(loginError);
      error.hidden = false;
    } finally {
      button.disabled = false;
    }
  });

  refreshButton.addEventListener("click", () => loadItems(true));

  document.getElementById("logout-button").addEventListener("click", async () => {
    try {
      await json(apiURL("auth", "logout"), { method: "POST", mutate: true });
    } catch (error) {}
    state.items = [];
    renderItems();
    show(boardView, false);
    show(authView, true);
  });

  const textMode = document.getElementById("text-mode");
  const fileMode = document.getElementById("file-mode");
  function setMode(file) {
    textMode.setAttribute("aria-selected", String(!file));
    fileMode.setAttribute("aria-selected", String(file));
    show(document.getElementById("text-form"), !file);
    show(document.getElementById("file-form"), file);
  }
  textMode.addEventListener("click", () => setMode(false));
  fileMode.addEventListener("click", () => setMode(true));

  document.getElementById("text-form").addEventListener("submit", async event => {
    event.preventDefault();
    const textarea = document.getElementById("text-content");
    const button = document.getElementById("text-submit");
    const content = textarea.value;
    if (!content.trim()) return showToast("文本不能为空");
    button.disabled = true;
    try {
      const item = await json(apiURL("items"), {
        method: "POST",
        body: JSON.stringify({ title: content.split("\n", 1)[0].slice(0, 80), content }),
        mutate: true
      });
      textarea.value = "";
      addItems([item]);
      showToast("上传完成");
    } catch (error) {
      showToast(userError(error));
    } finally {
      button.disabled = false;
    }
  });

  const fileInput = document.getElementById("file-input");
  const dropZone = document.getElementById("drop-zone");
  const selectedFiles = document.getElementById("selected-files");

  function renderFiles() {
    selectedFiles.replaceChildren();
    state.files.forEach(file => {
      const row = document.createElement("div");
      row.className = "break-words";
      row.textContent = `${file.name} · ${formatSize(file.size)}`;
      selectedFiles.append(row);
    });
  }

  function clearAttachmentState() {
    state.files = [];
    fileInput.value = "";
    dropZone.classList.remove("dragover");
    selectedFiles.replaceChildren();
  }

  fileInput.addEventListener("change", () => {
    state.files = Array.from(fileInput.files);
    renderFiles();
  });
  ["dragenter", "dragover"].forEach(type => dropZone.addEventListener(type, event => {
    event.preventDefault();
    dropZone.classList.add("dragover");
  }));
  ["dragleave", "drop"].forEach(type => dropZone.addEventListener(type, event => {
    event.preventDefault();
    dropZone.classList.remove("dragover");
  }));
  dropZone.addEventListener("drop", event => {
    state.files = Array.from(event.dataTransfer.files);
    renderFiles();
  });

  document.getElementById("file-form").addEventListener("submit", event => {
    event.preventDefault();
    if (!state.files.length) return showToast("请选择文件");

    const uploadedFiles = state.files;
    const body = new FormData();
    uploadedFiles.forEach(file => body.append("files", file, file.name));
    const upload = new XMLHttpRequest();
    upload.open("POST", apiURL("items"));
    upload.setRequestHeader("X-SusShare-Request", "1");

    const progress = document.createElement("div");
    progress.className = "progress";
    const bar = document.createElement("div");
    bar.className = "progress-bar";
    progress.append(bar);
    selectedFiles.replaceChildren(progress);

    const restoreSelection = () => {
      state.files = uploadedFiles;
      renderFiles();
    };
    const showUploadFailure = messages => {
      restoreSelection();
      messages.forEach(message => {
        const row = document.createElement("div");
        row.className = "text-sm text-red-700";
        row.textContent = message;
        selectedFiles.append(row);
      });
      showToast("上传失败");
    };

    upload.upload.addEventListener("progress", event => {
      if (event.total) bar.style.width = `${Math.round(event.loaded / event.total * 100)}%`;
    });
    upload.addEventListener("load", () => {
      let payload = { results: [] };
      try {
        payload = JSON.parse(upload.responseText || "{}");
      } catch (error) {}
      const results = payload.results || [];
      const successful = results.filter(result => result.status === "ok").map(result => result.item);
      addItems(successful);
      if (upload.status >= 200 && upload.status < 300 && results.length === successful.length && results.length > 0) {
        clearAttachmentState();
        showToast("上传完成");
        return;
      }
      const messages = results.length
        ? results.filter(result => result.status !== "ok").map(result => {
            const name = result.filename || "未命名文件";
            return `${name} · ${errorMessages.get(result.error) || "上传失败"}`;
          })
        : ["上传失败"];
      showUploadFailure(messages);
    });
    upload.addEventListener("error", () => showUploadFailure(["网络错误"]));
    upload.addEventListener("abort", () => showUploadFailure(["已取消"]));
    upload.send(body);
  });

  const settingsModal = document.getElementById("settings-modal");
  document.getElementById("settings-button").addEventListener("click", async () => {
    try {
      const settings = await json(apiURL("settings"));
      document.getElementById("retention-select").value = settings.retention;
      settingsModal.showModal();
    } catch (error) {
      showToast(userError(error));
    }
  });
  document.getElementById("settings-close").addEventListener("click", () => settingsModal.close());

  document.getElementById("settings-form").addEventListener("submit", async event => {
    event.preventDefault();
    try {
      await json(apiURL("settings"), {
        method: "PUT",
        body: JSON.stringify({ retention: document.getElementById("retention-select").value }),
        mutate: true
      });
      showToast("设置已保存");
      settingsModal.close();
    } catch (error) {
      showToast(userError(error));
    }
  });

  document.getElementById("password-form").addEventListener("submit", async event => {
    event.preventDefault();
    try {
      await json(apiURL("settings", "password"), {
        method: "POST",
        body: JSON.stringify({
          current_password: document.getElementById("current-password").value,
          new_password: document.getElementById("new-password").value
        }),
        mutate: true
      });
      settingsModal.close();
      show(boardView, false);
      show(authView, true);
      showToast("密码已修改，请重新登录");
    } catch (error) {
      showToast(userError(error));
    }
  });

  (async () => {
    try {
      const payload = await json(apiURL("items"));
      state.items = payload.items || [];
      renderItems();
      show(authView, false);
      show(boardView, true);
    } catch (error) {
      show(authView, true);
    }
  })();
})();
