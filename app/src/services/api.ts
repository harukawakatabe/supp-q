export type ServiceHealth = {
  status: "ok" | "degraded";
  service: string;
  version: string;
  requestId?: string;
};

const publicApiBase = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");

export async function getServiceHealth(): Promise<ServiceHealth> {
  const response = await uni.request({
    url: `${publicApiBase}/api/v1/health/live`,
    method: "GET",
    timeout: 3000,
  });

  if (response.statusCode !== 200) {
    throw new Error(`health endpoint returned ${response.statusCode}`);
  }

  if (
    typeof response.data !== "object" ||
    response.data === null ||
    !("status" in response.data) ||
    !("service" in response.data) ||
    !("version" in response.data)
  ) {
    throw new Error("health endpoint returned an invalid contract");
  }

  return response.data as ServiceHealth;
}
