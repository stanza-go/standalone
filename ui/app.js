// Stanza UI — vanilla JS SPA with user authentication.
// Pages: login, register, profile (with edit + password change).
// Auth: cookie-based JWT with 60s status polling.

// ---------------------------------------------------------------------------
// API client
// ---------------------------------------------------------------------------

async function api(method, path, body) {
  const opts = {
    method,
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(`/api${path}`, opts);
  const data = await res.json();
  if (!res.ok) {
    const err = new Error(data.error || "Something went wrong");
    err.status = res.status;
    err.fields = data.fields || null;
    throw err;
  }
  return data;
}

function applyFieldErrors(formEl, fields, idMap) {
  // Clear previous field errors.
  formEl.querySelectorAll(".field-error").forEach((el) => el.remove());
  formEl
    .querySelectorAll(".input-error")
    .forEach((el) => el.classList.remove("input-error"));

  if (!fields) return;
  for (const [field, msg] of Object.entries(fields)) {
    const id = idMap && idMap[field] ? idMap[field] : field;
    const input =
      formEl.querySelector(`#${CSS.escape(id)}`) ||
      formEl.querySelector(`[name="${CSS.escape(id)}"]`);
    if (input) {
      input.classList.add("input-error");
      const errorDiv = document.createElement("div");
      errorDiv.className = "field-error";
      errorDiv.textContent = msg;
      input.parentNode.insertBefore(errorDiv, input.nextSibling);
    }
  }
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

let currentUser = null;
let pollTimer = null;

// ---------------------------------------------------------------------------
// Auth helpers
// ---------------------------------------------------------------------------

async function checkStatus() {
  try {
    const data = await api("GET", "/auth/");
    currentUser = data.user;
  } catch {
    currentUser = null;
  }
}

function startPolling() {
  stopPolling();
  pollTimer = setInterval(() => {
    if (document.visibilityState === "visible") {
      checkStatus().then(render);
    }
  }, 60_000);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

// ---------------------------------------------------------------------------
// Router (hash-based)
// ---------------------------------------------------------------------------

function navigate(hash) {
  window.location.hash = hash;
}

function currentRoute() {
  return window.location.hash.slice(1) || "/";
}

// ---------------------------------------------------------------------------
// Render engine
// ---------------------------------------------------------------------------

const app = document.getElementById("app");

function render() {
  const route = currentRoute();

  if (currentUser === undefined) {
    app.innerHTML = '<div class="spinner">Loading...</div>';
    return;
  }

  // Redirect logic
  if (!currentUser && route !== "/login" && route !== "/register") {
    navigate("/login");
    return;
  }
  if (currentUser && (route === "/login" || route === "/register")) {
    navigate("/");
    return;
  }

  if (route === "/login") {
    renderLogin();
  } else if (route === "/register") {
    renderRegister();
  } else {
    renderProfile();
  }
}

// ---------------------------------------------------------------------------
// Login page
// ---------------------------------------------------------------------------

function renderLogin() {
  app.innerHTML = `
    <div class="auth-page">
      <div class="card">
        <div class="card-header">
          <h1>Welcome back</h1>
          <p>Sign in to your account</p>
        </div>
        <form id="login-form">
          <div id="login-error"></div>
          <div class="form-group">
            <label for="email">Email</label>
            <input id="email" type="email" required autocomplete="email" autofocus />
          </div>
          <div class="form-group">
            <label for="password">Password</label>
            <input id="password" type="password" required autocomplete="current-password" />
          </div>
          <button type="submit" class="btn btn-primary btn-block" id="login-btn">Sign in</button>
        </form>
        <p class="text-center text-muted mt-1">
          Don't have an account? <a href="#/register" class="link">Create one</a>
        </p>
      </div>
    </div>
  `;

  const form = document.getElementById("login-form");
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const btn = document.getElementById("login-btn");
    const errorEl = document.getElementById("login-error");
    errorEl.innerHTML = "";
    applyFieldErrors(form, null);
    btn.disabled = true;
    btn.textContent = "Signing in...";

    try {
      const data = await api("POST", "/auth/login", {
        email: document.getElementById("email").value,
        password: document.getElementById("password").value,
      });
      currentUser = data.user;
      startPolling();
      navigate("/");
    } catch (err) {
      if (err.fields) {
        applyFieldErrors(form, err.fields);
      } else {
        errorEl.innerHTML = `<div class="alert alert-error">${escapeHtml(err.message)}</div>`;
      }
      btn.disabled = false;
      btn.textContent = "Sign in";
    }
  });
}

// ---------------------------------------------------------------------------
// Register page
// ---------------------------------------------------------------------------

function renderRegister() {
  app.innerHTML = `
    <div class="auth-page">
      <div class="card">
        <div class="card-header">
          <h1>Create account</h1>
          <p>Get started in seconds</p>
        </div>
        <form id="register-form">
          <div id="register-error"></div>
          <div class="form-group">
            <label for="name">Name</label>
            <input id="name" type="text" autocomplete="name" autofocus />
          </div>
          <div class="form-group">
            <label for="email">Email</label>
            <input id="email" type="email" required autocomplete="email" />
          </div>
          <div class="form-group">
            <label for="password">Password</label>
            <input id="password" type="password" required autocomplete="new-password" minlength="8" />
          </div>
          <button type="submit" class="btn btn-primary btn-block" id="register-btn">Create account</button>
        </form>
        <p class="text-center text-muted mt-1">
          Already have an account? <a href="#/login" class="link">Sign in</a>
        </p>
      </div>
    </div>
  `;

  const form = document.getElementById("register-form");
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const btn = document.getElementById("register-btn");
    const errorEl = document.getElementById("register-error");
    errorEl.innerHTML = "";
    applyFieldErrors(form, null);
    btn.disabled = true;
    btn.textContent = "Creating account...";

    try {
      const data = await api("POST", "/auth/register", {
        name: document.getElementById("name").value,
        email: document.getElementById("email").value,
        password: document.getElementById("password").value,
      });
      currentUser = data.user;
      startPolling();
      navigate("/");
    } catch (err) {
      if (err.fields) {
        applyFieldErrors(form, err.fields);
      } else {
        errorEl.innerHTML = `<div class="alert alert-error">${escapeHtml(err.message)}</div>`;
      }
      btn.disabled = false;
      btn.textContent = "Create account";
    }
  });
}

// ---------------------------------------------------------------------------
// Profile page
// ---------------------------------------------------------------------------

function renderProfile() {
  app.innerHTML = `
    <div class="shell">
      <header class="shell-header">
        <h1>Stanza</h1>
        <nav>
          <span>${escapeHtml(currentUser.email)}</span>
          <button class="btn btn-ghost" id="logout-btn">Sign out</button>
        </nav>
      </header>
      <main class="shell-body">
        <div class="section" id="profile-section">
          <h2>Profile</h2>
          <div id="profile-content">Loading...</div>
        </div>
        <div class="section" id="edit-section" style="display:none">
          <h2>Edit Profile</h2>
          <form id="edit-form">
            <div id="edit-error"></div>
            <div id="edit-success"></div>
            <div class="form-group">
              <label for="edit-name">Name</label>
              <input id="edit-name" type="text" />
            </div>
            <div class="form-group">
              <label for="edit-email">Email</label>
              <input id="edit-email" type="email" />
            </div>
            <div class="btn-row">
              <button type="submit" class="btn btn-primary" id="edit-btn">Save changes</button>
              <button type="button" class="btn btn-secondary" id="edit-cancel">Cancel</button>
            </div>
          </form>
        </div>
        <div class="section" id="password-section" style="display:none">
          <h2>Change Password</h2>
          <form id="password-form">
            <div id="password-error"></div>
            <div id="password-success"></div>
            <div class="form-group">
              <label for="current-password">Current password</label>
              <input id="current-password" type="password" required autocomplete="current-password" />
            </div>
            <div class="form-group">
              <label for="new-password">New password</label>
              <input id="new-password" type="password" required autocomplete="new-password" minlength="8" />
            </div>
            <div class="btn-row">
              <button type="submit" class="btn btn-primary" id="password-btn">Update password</button>
              <button type="button" class="btn btn-secondary" id="password-cancel">Cancel</button>
            </div>
          </form>
        </div>
      </main>
    </div>
  `;

  // Logout
  document.getElementById("logout-btn").addEventListener("click", async () => {
    try {
      await api("POST", "/auth/logout");
    } catch {
      // Clear state regardless
    }
    currentUser = null;
    stopPolling();
    navigate("/login");
  });

  // Load profile
  loadProfile();

  // Edit form
  document.getElementById("edit-form").addEventListener("submit", handleEditProfile);
  document.getElementById("edit-cancel").addEventListener("click", () => {
    document.getElementById("edit-section").style.display = "none";
    document.getElementById("profile-section").style.display = "";
  });

  // Password form
  document.getElementById("password-form").addEventListener("submit", handleChangePassword);
  document.getElementById("password-cancel").addEventListener("click", () => {
    document.getElementById("password-section").style.display = "none";
    document.getElementById("profile-section").style.display = "";
  });
}

async function loadProfile() {
  const content = document.getElementById("profile-content");
  try {
    const data = await api("GET", "/user/profile");
    const u = data.user;
    currentUser = { id: u.id, email: u.email, name: u.name };

    content.innerHTML = `
      <div class="info-row">
        <span class="info-label">Name</span>
        <span>${escapeHtml(u.name || "—")}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Email</span>
        <span>${escapeHtml(u.email)}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Member since</span>
        <span>${formatDate(u.created_at)}</span>
      </div>
      <div class="btn-row">
        <button class="btn btn-secondary" id="show-edit">Edit profile</button>
        <button class="btn btn-secondary" id="show-password">Change password</button>
      </div>
    `;

    document.getElementById("show-edit").addEventListener("click", () => {
      document.getElementById("profile-section").style.display = "none";
      document.getElementById("edit-section").style.display = "";
      document.getElementById("edit-name").value = u.name || "";
      document.getElementById("edit-email").value = u.email;
      document.getElementById("edit-error").innerHTML = "";
      document.getElementById("edit-success").innerHTML = "";
    });

    document.getElementById("show-password").addEventListener("click", () => {
      document.getElementById("profile-section").style.display = "none";
      document.getElementById("password-section").style.display = "";
      document.getElementById("current-password").value = "";
      document.getElementById("new-password").value = "";
      document.getElementById("password-error").innerHTML = "";
      document.getElementById("password-success").innerHTML = "";
    });
  } catch (err) {
    content.innerHTML = `<div class="alert alert-error">${escapeHtml(err.message)}</div>`;
  }
}

async function handleEditProfile(e) {
  e.preventDefault();
  const form = document.getElementById("edit-form");
  const btn = document.getElementById("edit-btn");
  const errorEl = document.getElementById("edit-error");
  const successEl = document.getElementById("edit-success");
  errorEl.innerHTML = "";
  successEl.innerHTML = "";
  applyFieldErrors(form, null);
  btn.disabled = true;
  btn.textContent = "Saving...";

  try {
    const body = {};
    const name = document.getElementById("edit-name").value.trim();
    const email = document.getElementById("edit-email").value.trim();
    if (name) body.name = name;
    if (email) body.email = email;

    if (!name && !email) {
      throw new Error("At least one field is required");
    }

    const data = await api("PUT", "/user/profile", body);
    currentUser = { id: data.user.id, email: data.user.email, name: data.user.name };

    successEl.innerHTML = '<div class="alert alert-success">Profile updated</div>';
    btn.disabled = false;
    btn.textContent = "Save changes";
  } catch (err) {
    if (err.fields) {
      applyFieldErrors(form, err.fields, { email: "edit-email", name: "edit-name" });
    } else {
      errorEl.innerHTML = `<div class="alert alert-error">${escapeHtml(err.message)}</div>`;
    }
    btn.disabled = false;
    btn.textContent = "Save changes";
  }
}

async function handleChangePassword(e) {
  e.preventDefault();
  const form = document.getElementById("password-form");
  const btn = document.getElementById("password-btn");
  const errorEl = document.getElementById("password-error");
  const successEl = document.getElementById("password-success");
  errorEl.innerHTML = "";
  successEl.innerHTML = "";
  applyFieldErrors(form, null);
  btn.disabled = true;
  btn.textContent = "Updating...";

  try {
    await api("PUT", "/user/profile/password", {
      current_password: document.getElementById("current-password").value,
      new_password: document.getElementById("new-password").value,
    });

    successEl.innerHTML = '<div class="alert alert-success">Password updated</div>';
    document.getElementById("current-password").value = "";
    document.getElementById("new-password").value = "";
    btn.disabled = false;
    btn.textContent = "Update password";
  } catch (err) {
    if (err.fields) {
      applyFieldErrors(form, err.fields, {
        current_password: "current-password",
        new_password: "new-password",
      });
    } else {
      errorEl.innerHTML = `<div class="alert alert-error">${escapeHtml(err.message)}</div>`;
    }
    btn.disabled = false;
    btn.textContent = "Update password";
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function escapeHtml(str) {
  const d = document.createElement("div");
  d.textContent = str;
  return d.innerHTML;
}

function formatDate(iso) {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

// ---------------------------------------------------------------------------
// Boot
// ---------------------------------------------------------------------------

currentUser = undefined; // loading state

window.addEventListener("hashchange", render);

checkStatus().then(() => {
  if (currentUser) {
    startPolling();
  }
  render();
});
