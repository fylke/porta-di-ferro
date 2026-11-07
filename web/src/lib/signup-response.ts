import { signupSubmissionId } from './signup-id';

/**
 * A signup response as the organizer's browser makes one: read from a filled PDF form
 * (signup-pdf.ts), or typed in by hand for one that could not be read (Signup.svelte). The
 * participant's own app writes the same file; internal/signup reads all of them
 * (docs/proposals/offline-signup.md).
 */

export const RESPONSE_FORMAT = 'porta.signup.response';
export const RESPONSE_VERSION = 2;

export interface ResponseFields {
  definitionId: string;
  name: string;
  club: string;
  email: string;
  entries: string[];
  /** Left out when the response offers no work. */
  staff?: { tournaments: string[]; roles: string[] };
}

/**
 * The response. Its submission id comes from the event and the email, so the same person's
 * response is the same submission however many times it is read.
 */
export function signupResponse(f: ResponseFields) {
  const email = f.email.trim().toLowerCase();
  return {
    format: RESPONSE_FORMAT,
    version: RESPONSE_VERSION,
    definitionId: f.definitionId.trim(),
    submissionId: signupSubmissionId(f.definitionId, email),
    participant: { name: f.name, club: f.club, email },
    entries: f.entries,
    ...(f.staff ? { staff: f.staff } : {}),
  };
}
