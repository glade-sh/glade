export async function readJSONResponse(response, label) {
  const body = await response.text()
  try {
    return JSON.parse(body)
  } catch (error) {
    throw new Error(`${label} did not return parseable JSON: ${error.message}`)
  }
}
