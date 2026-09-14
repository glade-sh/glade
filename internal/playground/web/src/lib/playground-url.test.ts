import { describe, expect, it } from "vitest"

import { canonicalPlaygroundURL, parsePlaygroundURL } from "@/lib/playground-url"

describe("playground URLs", () => {
  it("round-trips a surface, example, and active file", () => {
    const url = canonicalPlaygroundURL("https://try.glade.sh/playground/?old=1#stale", {
      surface: "apex",
      example: "refinement-service",
      file: "force-app/main/default/classes/RefinementService.cls",
    })

    expect(url.toString()).toBe(
      "https://try.glade.sh/playground/?surface=apex&example=refinement-service&file=force-app%2Fmain%2Fdefault%2Fclasses%2FRefinementService.cls",
    )
    expect(parsePlaygroundURL(url.toString())).toEqual({
      surface: "apex",
      example: "refinement-service",
      file: "force-app/main/default/classes/RefinementService.cls",
    })
  })

  it("keeps legacy example hashes and rejects unknown surfaces", () => {
    expect(parsePlaygroundURL("http://localhost/playground/#example=collection-selector")).toEqual({
      surface: "apex",
      example: "collection-selector",
    })
    expect(parsePlaygroundURL("http://localhost/playground/?surface=unknown").surface).toBe("apex")
  })
})
