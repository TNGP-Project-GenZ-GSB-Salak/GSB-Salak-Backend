const API_BASE = "http://localhost:8080/api/v1";

// Stored under its own key, distinct from the customer frontends' "token" -
// this admin tool is a different credential system entirely (see
// internal/admin and internal/platform/jwtutil/admin.go's separate secret),
// not just a different value under the same name.
function getAdminToken() {
  return localStorage.getItem("adminToken");
}

function setAdminToken(token) {
  localStorage.setItem("adminToken", token);
}

function clearAdminToken() {
  localStorage.removeItem("adminToken");
}

function requireAdminAuth() {
  if (!getAdminToken()) {
    window.location.href = "login.html";
  }
}

async function apiFetch(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  const token = getAdminToken();
  if (token) {
    headers["Authorization"] = "Bearer " + token;
  }

  const res = await fetch(API_BASE + path, { ...options, headers });
  const body = await res.json().catch(() => ({}));

  if (!res.ok) {
    throw new Error(body.error || `Request failed with status ${res.status}`);
  }
  return body.data;
}

function showError(el, err) {
  el.textContent = err.message || String(err);
  el.classList.remove("hidden");
}

function clearMessage(el) {
  el.textContent = "";
  el.classList.add("hidden");
}
