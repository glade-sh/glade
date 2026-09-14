import { describe, expect, it } from "vitest"

import { apiResponseError, friendlyErrorMessage, PlaygroundAPIError } from "@/lib/api-error"

describe("apiResponseError", () => {
  it("keeps a structured timeout message from a non-successful run", () => {
    const response = new Response("", { status: 503, statusText: "Service Unavailable" })
    const error = apiResponseError(response, {
      errorMessage: "execution timed out after 5s; try a smaller example",
    })

    expect(error).toBeInstanceOf(PlaygroundAPIError)
    expect(friendlyErrorMessage(error)).toBe("Execution timed out after 5s; try a smaller example")
  })

  it("turns a rate limit response into actionable copy", () => {
    const response = new Response("", { status: 429, headers: { "Retry-After": "60" } })

    expect(friendlyErrorMessage(apiResponseError(response, { error: "rate limit exceeded" }))).toBe(
      "Too many playground actions. Try again in about 60 seconds.",
    )
  })
})
