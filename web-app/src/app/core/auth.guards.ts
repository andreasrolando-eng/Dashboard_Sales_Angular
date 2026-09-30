import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';
import { Auth } from './auth';

/** Only signed-in users may enter; everyone else is sent to the login form. */
export const authGuard: CanActivateFn = async (_route, state) => {
    const auth = inject(Auth);
    const router = inject(Router);
    if (await auth.ensureLoaded()) return true;
    return router.createUrlTree(['/login'], state.url && state.url !== '/' ? { queryParams: { returnUrl: state.url } } : {});
};

/** Admin-only pages (Kelola User, Sinkron Data). Others go back to Overview. */
export const adminGuard: CanActivateFn = async () => {
    const auth = inject(Auth);
    const router = inject(Router);
    const me = await auth.ensureLoaded();
    if (me?.is_admin) return true;
    return router.createUrlTree([me ? '/' : '/login']);
};

/** The login page itself: skip it when already signed in. */
export const guestGuard: CanActivateFn = async () => {
    const auth = inject(Auth);
    const router = inject(Router);
    return (await auth.ensureLoaded()) ? router.createUrlTree(['/']) : true;
};

/**
 * If an API call says 401 while the app thinks we're signed in, the session
 * ended (expired, logged out elsewhere, password changed): go to the form.
 * The sign-in and "who am I" calls are exempt -- their 401 is an answer, not
 * an expiry.
 */
export const unauthorizedInterceptor: HttpInterceptorFn = (req, next) => {
    const auth = inject(Auth);
    return next(req).pipe(
        catchError((err) => {
            if (err?.status === 401 && !req.url.endsWith('/api/auth/login') && !req.url.endsWith('/api/me')) {
                auth.sessionExpired();
            }
            return throwError(() => err);
        }),
    );
};
