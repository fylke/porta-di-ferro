import { t } from './i18n.svelte';

export async function pdfSignupJSON(data: ArrayBuffer): Promise<string> {
    if (data.byteLength > 2 * 1024 * 1024) {
        throw new Error(t('PDF signup files must not exceed 2 MB.'));
    }
    const { PDFDocument, PDFTextField, PDFCheckBox } = await import('pdf-lib');
    let fields;
    try {
        const document = await PDFDocument.load(data);
        fields = document.getForm().getFields();
    } catch {
        throw new Error(t('Cannot read PDF form fields. Return the saved fillable form, not a scan or printed copy.'));
    }
    const values = new Map<string, string>();
    const entries: string[] = [];
    const tournaments: string[] = [];
    const roles: string[] = [];
    for (const field of fields) {
        const name = field.getName();
        if (values.has(name)) {
            throw new Error(t('Not a supported Porta signup PDF. Ask the organizer for a new form.'));
        }
        values.set(name, field instanceof PDFTextField ? (field.getText() ?? '') : '');
        if (!(field instanceof PDFCheckBox) || !field.isChecked()) continue;
        if (name.startsWith('entry-')) entries.push(name.slice('entry-'.length));
        if (name.startsWith('staff-')) tournaments.push(name.slice('staff-'.length));
        if (name.startsWith('role-')) roles.push(name.slice('role-'.length));
    }
    const definitionId = values.get('porta-event')?.trim();
    if (values.get('porta-format') !== 'porta.signup.response/2' || !definitionId) {
        throw new Error(t('Not a supported Porta signup PDF. Ask the organizer for a new form.'));
    }
    const email = (values.get('email') ?? '').trim().toLowerCase();
    return JSON.stringify({
        format: 'porta.signup.response',
        version: 2,
        definitionId,
        submissionId: `pdf-${JSON.stringify([definitionId, email])}`,
        participant: { name: values.get('name') ?? '', club: values.get('club') ?? '', email },
        entries,
        staff: { tournaments, roles },
    });
}