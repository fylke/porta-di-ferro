import { sha256 } from '@noble/hashes/sha256';
import { bytesToHex } from '@noble/hashes/utils';

export function signupSubmissionId(definitionId: string, email: string): string {
    const identity = `${definitionId.trim()}\0${email.trim().toLowerCase()}`;
    return `pdf-${bytesToHex(sha256(new TextEncoder().encode(identity)))}`;
}
