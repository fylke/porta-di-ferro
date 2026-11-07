/** What went wrong, for the page to say: an error's message, or whatever was thrown. */
export function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}
