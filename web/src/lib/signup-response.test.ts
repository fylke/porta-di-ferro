import { describe, expect, it } from 'vitest';
import { signupSubmissionId } from './signup-id';
import { signupResponse } from './signup-response';

describe('a signup response made in the browser', () => {
  it('is the participant app’s format, named by the event and the email', () => {
    const r = signupResponse({ definitionId: ' open-2026 ', name: 'Åsa', club: 'MSL', email: ' ASA@Example.org ', entries: ['open-sabre'] });
    expect(r).toEqual({
      format: 'porta.signup.response',
      version: 2,
      definitionId: 'open-2026',
      submissionId: signupSubmissionId('open-2026', 'asa@example.org'),
      participant: { name: 'Åsa', club: 'MSL', email: 'asa@example.org' },
      entries: ['open-sabre'],
    });
    expect(JSON.stringify(r)).not.toContain('staff');
  });

  it('carries an offer to work when there is one', () => {
    const r = signupResponse({ definitionId: 'e', name: 'Bo', club: '', email: 'bo@x', entries: [], staff: { tournaments: ['open-sabre'], roles: ['head-ref'] } });
    expect(r.staff).toEqual({ tournaments: ['open-sabre'], roles: ['head-ref'] });
  });
});
