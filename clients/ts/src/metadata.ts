/**
 * Encode metadata for `metadata_json` on the wire (base64 of UTF-8 JSON), matching Go's JSON unmarshaling into `[]byte`.
 */
export function metadataToJsonField(meta: Record<string, unknown> | undefined): string | undefined {
  if (meta == null || Object.keys(meta).length === 0) {
    return undefined;
  }
  const json = JSON.stringify(meta);
  const bytes = new TextEncoder().encode(json);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]!);
  }
  return btoa(binary);
}
