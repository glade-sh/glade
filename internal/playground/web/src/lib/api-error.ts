export class PlaygroundAPIError extends Error {
  readonly status: number
  readonly retryAfterSeconds: number | null

  constructor(message: string, status: number, retryAfterSeconds: number | null = null) {
    super(message)
    this.name = "PlaygroundAPIError"
    this.status = status
    this.retryAfterSeconds = retryAfterSeconds
  }
}
function responseMessage(body: unknown, fallback: string) {
  if (typeof body !== "object" || !body) return fallback
  if ("errorMessage" in body && typeof body.errorMessage === "string" && body.errorMessage.trim()) {
    return body.errorMessage.trim()
  }
  if ("error" in body && typeof body.error === "string" && body.error.trim()) {
    return body.error.trim()
  }
  return fallback
}

export function apiResponseError(response: Response, body: unknown) {
  const retryAfter = Number.parseInt(response.headers.get("Retry-After") ?? "", 10)
  return new PlaygroundAPIError(
    responseMessage(body, response.statusText || `Request failed (${response.status})`),
    response.status,
    Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : null,
  )
}

export function friendlyErrorMessage(error: unknown) {
  if (error instanceof PlaygroundAPIError) {
    if (error.status === 429) {
      const wait = error.retryAfterSeconds ? ` Try again in about ${error.retryAfterSeconds} seconds.` : " Try again shortly."
      return `Too many playground actions.${wait}`
    }
    if (error.status === 409) {
      return "This file changed in another session. Restore the example, then try your edit again."
    }
    if (error.status === 413) {
      return "This workspace reached its public size limit. Restore the example or remove a file before continuing."
    }
    if (error.message.toLowerCase().includes("timed out")) {
      return error.message.charAt(0).toUpperCase() + error.message.slice(1)
    }
    if (error.status === 503) {
      return "The playground runtime is not ready. Try again shortly."
    }
    return error.message
  }
  if (error instanceof TypeError) {
    return "The playground service could not be reached. Check the server and try again."
  }
  if (error instanceof Error && error.message.trim()) return error.message
  return String(error)
}
