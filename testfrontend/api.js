const API_BASE = "http://localhost:8080/api/v1";

function getToken() {
  return localStorage.getItem("token");
}

function setToken(token) {
  localStorage.setItem("token", token);
}

function clearToken() {
  localStorage.removeItem("token");
}

function requireAuth() {
  if (!getToken()) {
    window.location.href = "login.html";
  }
}

async function apiFetch(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  const token = getToken();
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
