const idr = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 0 });
const dec = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 });

export const rupiah = (n: number | null | undefined): string => (n == null ? '–' : 'Rp ' + idr.format(Math.round(n)));
export const num = (n: number | null | undefined): string => (n == null ? '–' : idr.format(n));
export const pct = (n: number | null | undefined): string => (n == null ? '–' : dec.format(n) + '%');
export const shortDate = (iso: string): string =>
    new Date(iso).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', timeZone: 'UTC' });
export const longDate = (iso: string): string =>
    new Date(iso).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric', timeZone: 'UTC' });
export const duration = (seconds: number | null | undefined): string => {
    if (!seconds) return '–';
    const m = Math.round(seconds / 60);
    return m >= 60 ? `${Math.floor(m / 60)} j ${m % 60} m` : `${m} menit`;
};

export const ratio = (n: number | null | undefined): string => (n == null ? '–' : new Intl.NumberFormat('id-ID', { maximumFractionDigits: 2 }).format(n) + '×');

/** Short axis label: 1.250.000 -> "1,3 jt", 250.000 -> "250 rb". */
export const compact = (n: number): string => {
    const a = Math.abs(n);
    const d = new Intl.NumberFormat('id-ID', { maximumFractionDigits: 1 });
    if (a >= 1e9) return d.format(n / 1e9) + ' M';
    if (a >= 1e6) return d.format(n / 1e6) + ' jt';
    if (a >= 1e3) return d.format(n / 1e3) + ' rb';
    return d.format(n);
};

/** "29 Sep, 13.26" in WIB, for timestamps coming from the API. */
export const wibTime = (iso: string | null | undefined): string =>
    iso
        ? new Date(iso).toLocaleString('id-ID', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' })
        : '–';
