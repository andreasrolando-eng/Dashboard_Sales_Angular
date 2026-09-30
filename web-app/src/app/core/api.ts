import { httpResource } from '@angular/common/http';
import { inject } from '@angular/core';
import { Filters } from './filters';

type Extra = Record<string, string | number | undefined>;

/**
 * Reactive GET against the Go API. Re-fetches whenever the global date range
 * or outlet changes (or any signal read inside `extra`). Pass `useOutlet:
 * false` for endpoints that ignore the outlet filter, and `dated: false` for
 * endpoints without a date range.
 */
export function apiResource<T>(
    path: string,
    opts: { extra?: () => Extra; useOutlet?: boolean; dated?: boolean } = {},
) {
    const filters = inject(Filters);
    const { extra, useOutlet = true, dated = true } = opts;
    return httpResource<T>(() => {
        if (dated && !filters.valid()) return undefined;
        const params: Record<string, string | number> = {};
        if (dated) Object.assign(params, filters.range());
        if (useOutlet && filters.outlet()) params['outlet'] = filters.outlet();
        for (const [k, v] of Object.entries(extra?.() ?? {})) if (v !== undefined && v !== '') params[k] = v;
        return { url: '/api' + path, params };
    });
}
