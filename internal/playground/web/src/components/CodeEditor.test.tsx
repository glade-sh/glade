import { renderToString } from "react-dom/server"
import { expect, test, vi } from "vitest"

import { CodeEditor } from "./CodeEditor"

test("renders the editor textarea read-only when requested", () => {
  const html = renderToString(
    <CodeEditor title="Apex Source" value="public class Locked {}" onChange={vi.fn()} readOnly />,
  )

  expect(html).toContain("readOnly")
})
