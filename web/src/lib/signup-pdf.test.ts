import { describe, expect, it } from 'vitest';
import { PDFDocument } from 'pdf-lib';
import { pdfSignupJSON } from './signup-pdf';

async function savedPDF(email = ' ASA@Example.org ') {
    const document = await PDFDocument.create();
    const page = document.addPage();
    const form = document.getForm();
    for (const [name, value] of Object.entries({
        'porta-event': 'open-2026',
        'porta-format': 'porta.signup.response/2',
        name: 'Åsa Öberg',
        club: 'Fäktklubben',
        email,
    })) {
        form.createTextField(name).setText(value);
    }
    for (const [index, name] of ['entry-longsword', 'entry-sabre', 'staff-sabre', 'role-head-ref', 'role-physician'].entries()) {
        const checkbox = form.createCheckBox(name);
        checkbox.addToPage(page, { x: 40, y: 40 + index * 30, width: 18, height: 18 });
        if (name !== 'entry-sabre' && name !== 'role-physician') checkbox.check();
    }
    return (await document.save()).slice().buffer as ArrayBuffer;
}

describe('PDF to signup JSON', () => {
    it('converts saved text fields and checked boxes into a signup response', async () => {
        const response = JSON.parse(await pdfSignupJSON(await savedPDF()));
        expect(response).toMatchObject({
            format: 'porta.signup.response', version: 2, definitionId: 'open-2026',
            participant: { name: 'Åsa Öberg', club: 'Fäktklubben', email: 'asa@example.org' },
            entries: ['longsword'],
            staff: { tournaments: ['sabre'], roles: ['head-ref'] },
        });
    });

    it('keeps the same submission identity across copies and email normalization', async () => {
        const first = JSON.parse(await pdfSignupJSON(await savedPDF()));
        const copy = JSON.parse(await pdfSignupJSON(await savedPDF('asa@example.org')));
        const other = JSON.parse(await pdfSignupJSON(await savedPDF('other@example.org')));
        expect(copy.submissionId).toBe(first.submissionId);
        expect(other.submissionId).not.toBe(first.submissionId);
    });

    it('rejects PDFs without signup fields, including flattened forms', async () => {
        const document = await PDFDocument.create();
        await expect(pdfSignupJSON((await document.save()).slice().buffer as ArrayBuffer))
            .rejects.toThrow('Not a supported Porta signup PDF');
    });

    it('rejects unreadable and oversized files', async () => {
        await expect(pdfSignupJSON(new ArrayBuffer(0))).rejects.toThrow('Cannot read PDF form fields');
        await expect(pdfSignupJSON(new ArrayBuffer(2 * 1024 * 1024 + 1))).rejects.toThrow('2 MB');
    });
});