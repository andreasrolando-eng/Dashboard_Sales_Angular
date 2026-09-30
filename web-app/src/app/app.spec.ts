import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideTaiga } from '@taiga-ui/core';
import { App } from './app';

describe('App', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [App],
            providers: [provideRouter([]), provideTaiga()],
        }).compileComponents();
    });

    it('should create the app', () => {
        const fixture = TestBed.createComponent(App);
        expect(fixture.componentInstance).toBeTruthy();
    });

    it('renders only the router outlet inside the themed wrapper (the shell and login live in routes)', async () => {
        const fixture = TestBed.createComponent(App);
        await fixture.whenStable();
        const el = fixture.nativeElement as HTMLElement;
        const wrapper = el.querySelector('.app') as HTMLElement;
        expect(wrapper.dataset['theme']).toMatch(/^(light|dark)$/);
        expect(wrapper.querySelector('router-outlet')).toBeTruthy();
        expect(el.querySelector('.sidebar')).toBeNull(); // no chrome without a signed-in route
    });
});
