export class HttpError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "HttpError";
    this.status = status;
  }
}

async function parseJsonSafe(response: Response) {
  try {
    return (await response.json()) as unknown;
  } catch {
    return null;
  }
}

export async function requestJson<T>(input: RequestInfo | URL, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init);
  const payload = await parseJsonSafe(response);

  if (!response.ok) {
    let message = "请求失败";
    if (payload && typeof payload === "object" && "error" in payload) {
      const errorMessage = (payload as { error?: unknown }).error;
      if (typeof errorMessage === "string" && errorMessage) {
        message = errorMessage;
      }
    }
    throw new HttpError(response.status, message);
  }

  return payload as T;
}
