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

// Carries status/code (mirrors GSB-Salak-Frontend/src/lib/api.ts's own
// ApiError) so messageForError below can map a backend error to real Thai
// copy instead of showing the raw (English) backend string.
class ApiError extends Error {
  constructor(message, status, code) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
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
    throw new ApiError(body.error || `Request failed with status ${res.status}`, res.status, body.code);
  }
  return body.data;
}

// Thai copy for this admin domain's own error codes
// (internal/transaction/errorcodes.go's .WithCode(...) call sites reached
// through SettleMaturedHolding) - mirrors
// GSB-Salak-Frontend/src/lib/kapookErrorMessages.ts's MESSAGE_BY_CODE
// convention: a mapped code gets real Thai copy, anything else falls back
// to the generic per-status message below rather than the raw backend
// string.
const MESSAGE_BY_CODE = {
  transaction_no_primary_account: "ไม่พบบัญชีคู่โอนหลักของลูกค้ารายนี้",
  transaction_holding_already_settled: "สลากรายการนี้ครบกำหนดและทำรายการไปแล้ว",
};

// Per-Kind generic fallback, keyed by HTTP status - same six statuses
// apperror.HTTPStatus derives from Kind, same Thai copy as
// kapookErrorMessages.ts's MESSAGE_BY_STATUS.
const MESSAGE_BY_STATUS = {
  400: "คำขอไม่ถูกต้อง กรุณาตรวจสอบข้อมูลอีกครั้ง",
  401: "ไม่สามารถยืนยันตัวตนได้ กรุณาเข้าสู่ระบบใหม่อีกครั้ง",
  403: "คุณไม่มีสิทธิ์ทำรายการนี้",
  404: "ไม่พบข้อมูลที่ร้องขอ",
  409: "ไม่สามารถทำรายการนี้ได้ในขณะนี้ กรุณาลองใหม่อีกครั้ง",
  500: "เกิดข้อผิดพลาด กรุณาลองใหม่อีกครั้ง",
};

const DEFAULT_FALLBACK = "ทำรายการไม่สำเร็จ";

function messageForError(err, fallback = DEFAULT_FALLBACK) {
  if (err instanceof ApiError) {
    if (err.code && MESSAGE_BY_CODE[err.code]) return MESSAGE_BY_CODE[err.code];
    return MESSAGE_BY_STATUS[err.status] || MESSAGE_BY_STATUS[500];
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

// GET /admin/kapook/goals/stuck - read-only, no side effects. Every goal
// the worker has failed to auto-purchase at least once since its last
// success, most-attempts-first.
function listStuckKapookGoals() {
  return apiFetch("/admin/kapook/goals/stuck");
}

function showMessage(el, text) {
  el.textContent = text;
  el.classList.remove("hidden");
}

function clearMessage(el) {
  el.textContent = "";
  el.classList.add("hidden");
}
