export type ServiceHealth = {
  status: "ok" | "degraded";
  service: string;
  version: string;
  requestId?: string;
};

export type Actor = {
  userId: string;
  kind: "demo_ephemeral" | "registered";
  role: "member" | "admin";
  email?: string;
  workspaceId: string;
  workspaceKind: "demo" | "registered";
  workspaceName: string;
  expiresAt?: string;
};

export type Invitation = {
  id: string;
  kind: "generic_code" | "email_bound";
  email?: string;
  maxUses: number;
  useCount: number;
  expiresAt: string;
  revokedAt?: string;
  createdAt: string;
};

export class APIError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

const publicApiBase = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");

async function apiRequest<T>(path: string, method: "GET" | "POST" | "PUT" | "DELETE" = "GET", data?: object, headers?: Record<string, string>): Promise<T> {
  const response = await uni.request({
    url: `${publicApiBase}/api/v1${path}`,
    method,
    data,
    timeout: 8000,
    header: { ...(data ? { "Content-Type": "application/json" } : {}), ...headers },
    withCredentials: true,
  });

  if (response.statusCode < 200 || response.statusCode >= 300) {
    const payload = response.data as { error?: { code?: string; message?: string } } | undefined;
    throw new APIError(
      response.statusCode,
      payload?.error?.code ?? "request_failed",
      payload?.error?.message ?? `请求失败（${response.statusCode}）`,
    );
  }
  return response.data as T;
}

export function getServiceHealth(): Promise<ServiceHealth> {
  return apiRequest<ServiceHealth>("/health/live");
}

export async function getSession(): Promise<Actor> {
  const result = await apiRequest<{ actor: Actor }>("/session");
  return result.actor;
}

export function requestEmailCode(email: string, invitation: string): Promise<{ status: string }> {
  return apiRequest("/auth/email-code/request", "POST", { email, invitation });
}

export async function verifyEmailCode(email: string, code: string, invitation: string, password: string): Promise<Actor> {
  const result = await apiRequest<{ actor: Actor }>("/auth/email-code/verify", "POST", { email, code, invitation, password });
  return result.actor;
}

export async function passwordLogin(email: string, password: string): Promise<Actor> {
  const result = await apiRequest<{ actor: Actor }>("/auth/password/login", "POST", { email, password });
  return result.actor;
}

export function requestPasswordReset(email: string): Promise<{ status: string }> {
  return apiRequest("/auth/password-reset/request", "POST", { email });
}

export function confirmPasswordReset(email: string, code: string, password: string): Promise<{ status: string }> {
  return apiRequest("/auth/password-reset/confirm", "POST", { email, code, password });
}

export function logout(): Promise<void> {
  return apiRequest("/auth/logout", "POST");
}

export function deleteAccount(confirmation: string): Promise<{ status: string }> {
  return apiRequest("/account", "DELETE", { confirmation });
}

export async function listInvitations(): Promise<Invitation[]> {
  const result = await apiRequest<{ items: Invitation[] }>("/admin/invitations");
  return result.items;
}

export function createInvitation(input: { kind: string; email?: string; maxUses: number; expiresAt: string }): Promise<Invitation & { secret: string }> {
  return apiRequest("/admin/invitations", "POST", input);
}

export function revokeInvitation(id: string): Promise<void> {
  return apiRequest(`/admin/invitations/${encodeURIComponent(id)}`, "DELETE");
}

export type Schedule = {
  version: number;
  startDate: string;
  weekdays: number[];
  dayCycle: { enabled: boolean; cycleDays: number; takeDays: number; anchorDate: string };
  dayCycleHistory?: Array<{ effectiveDate: string; enabled: boolean; cycleDays: number; takeDays: number; anchorDate: string }>;
  longCycle: { enabled: boolean; takeWeeks: number; restWeeks: number; startDate: string };
  reminderTimes: string[];
};

export type Product = {
  id: string;
  name: string;
  brand: string;
  productType: "supplement" | "otc" | "prescription";
  status: "active" | "paused" | "depleted";
  unit: string;
  doseQuantity: number;
  doseTimesPerDay: number;
  dailyQuantity: number;
  ingredientServingQuantity: number;
  withFood?: boolean;
  currentQuantity: number;
  restockThresholdDays: number;
  expiryReminderDays: number;
  schedule: Schedule;
  batches: Array<{ id: string; initialQuantity: number; currentQuantity: number; expiryDate?: string; priceCny: number; createdAt: string }>;
  ingredients: Array<{ id: string; key: string; name: string; amount: number; unit: string }>;
  expiryRisk: { level: "none" | "safe" | "warn" | "danger"; message: string; expiryDate?: string; projectedFinishDate?: string; latestStartDate?: string };
  createdAt: string;
  updatedAt: string;
};

export type TodayItem = {
  product: Product;
  scheduledQuantity: number;
  takenQuantity: number;
  done: boolean;
  available: boolean;
  lastIntakeId?: string;
};

export type Intake = {
  id: string;
  productId: string;
  date: string;
  time?: string;
  quantity: number;
  source: string;
  status: "active" | "revoked";
  note?: string;
};

export type IntakeRecord = Intake & { productName: string; productUnit: string };

export async function getToday(date: string): Promise<TodayItem[]> {
  const result = await apiRequest<{ items: TodayItem[] }>(`/today?date=${encodeURIComponent(date)}`);
  return result.items;
}

export async function listProducts(): Promise<Product[]> {
  const result = await apiRequest<{ items: Product[] }>("/products");
  return result.items;
}

export function getProduct(id: string): Promise<Product> {
  return apiRequest(`/products/${encodeURIComponent(id)}`);
}

export type UpdateProductInput = {
  name: string;
  brand: string;
  productType: Product["productType"];
  status: "active" | "paused" | "depleted";
  unit: string;
  doseQuantity: number;
  doseTimesPerDay: number;
  ingredientServingQuantity: number;
  withFood?: boolean;
  restockThresholdDays: number;
  expiryReminderDays: number;
  schedule: Omit<Schedule, "version" | "dayCycleHistory">;
  ingredients: Array<{ key: string; name: string; amount: number; unit: string }>;
  effectiveDate: string;
};

export function updateProduct(id: string, input: UpdateProductInput): Promise<Product> {
  return apiRequest(`/products/${encodeURIComponent(id)}`, "PUT", input);
}

export async function listIntakes(from: string, to: string): Promise<IntakeRecord[]> {
  const result = await apiRequest<{ items: IntakeRecord[] }>(`/intakes?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`);
  return result.items;
}

export function createProduct(input: object): Promise<Product> {
  return apiRequest("/products", "POST", input);
}

export function addBatch(productId: string, input: { quantity: number; expiryDate?: string; priceCny?: number }): Promise<Product> {
  return apiRequest(`/products/${encodeURIComponent(productId)}/batches`, "POST", input);
}

export async function createIntake(input: { productId: string; date: string; time: string; quantity: number; source: string; note?: string }, idempotencyKey: string): Promise<{ intake: Intake; product: Product }> {
  return apiRequest("/intakes", "POST", input, { "Idempotency-Key": idempotencyKey });
}

export function undoIntake(id: string): Promise<{ intake: Intake; product: Product }> {
  return apiRequest(`/intakes/${encodeURIComponent(id)}`, "DELETE");
}

export type RecognitionRole = "front" | "facts" | "expiry";
export type RecognitionCandidate = { status: "recognized" | "partial" | "unrecognized"; language?: string; confidence: number; rawText?: string; raw?: string; date?: string; fields?: Record<string, unknown> };
export type RecognitionStage = { provider: string; model: string; durationMs: number };
export type RecognitionTrace = { mode: "fake" | "ocr_llm" | "direct_vl" | "dual"; selectedRoute: string; ocr?: RecognitionStage; structure?: RecognitionStage; direct?: RecognitionStage; directCandidate?: RecognitionCandidate };
export type OCREvidence = { rawText: string; provider: string; model: string; durationMs: number; completedAt: string };
export type RecognitionJob = { id: string; role: RecognitionRole; status: "queued" | "running" | "succeeded" | "partial" | "failed" | "cancelled"; provider: string; attempt: number; maxAttempts: number; confidence: number; result?: RecognitionCandidate; ocrEvidence?: OCREvidence; trace?: RecognitionTrace; errorCode?: string; errorMessage?: string };
export type RecognitionSet = { id: string; status: "processing" | "awaiting_confirmation" | "confirmed" | "cancelled"; productId?: string; files: Array<{ id: string; role: RecognitionRole; mimeType: string; byteSize: number }>; jobs: RecognitionJob[] };

export async function uploadRecognitionSet(files: Record<RecognitionRole, { path: string; name: string }>): Promise<RecognitionSet> {
  // #ifdef H5
  const form = new FormData();
  for (const role of ["front", "facts", "expiry"] as RecognitionRole[]) {
    const response = await fetch(files[role].path);
    const blob = await response.blob();
    form.append(role, blob, files[role].name || `${role}.jpg`);
  }
  const response = await fetch(`${publicApiBase}/api/v1/recognition/sets`, { method: "POST", body: form, credentials: "include" });
  const payload = await response.json() as RecognitionSet & { error?: { code?: string; message?: string } };
  if (!response.ok) throw new APIError(response.status, payload.error?.code ?? "upload_failed", payload.error?.message ?? "图片上传失败。");
  return payload;
  // #endif
  // #ifndef H5
  throw new APIError(501, "platform_upload_pending", "小程序上传适配器尚未实现，请先使用 H5。");
  // #endif
}

export function getRecognitionSet(id: string): Promise<RecognitionSet> {
  return apiRequest(`/recognition/sets/${encodeURIComponent(id)}`);
}

export function retryRecognitionJob(id: string): Promise<RecognitionSet> {
  return apiRequest(`/recognition/jobs/${encodeURIComponent(id)}/retry`, "POST");
}

export function confirmRecognitionSet(id: string, product: object): Promise<Product> {
  return apiRequest(`/recognition/sets/${encodeURIComponent(id)}/confirm`, "POST", product);
}
