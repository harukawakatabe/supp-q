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

async function apiRequest<T>(path: string, method: "GET" | "POST" | "DELETE" = "GET", data?: object): Promise<T> {
  const response = await uni.request({
    url: `${publicApiBase}/api/v1${path}`,
    method,
    data,
    timeout: 8000,
    header: data ? { "Content-Type": "application/json" } : undefined,
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
