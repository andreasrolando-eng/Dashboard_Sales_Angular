import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideTaiga } from '@taiga-ui/core';
import { CategoryFilter } from './category-filter';

@Component({
    imports: [CategoryFilter],
    template: `<app-category-filter [(category)]="category" [(detail)]="detail" />`,
})
class Host {
    category = signal('');
    detail = signal('');
}

describe('CategoryFilter', () => {
    const setup = async () => {
        await TestBed.configureTestingModule({
            imports: [Host],
            providers: [provideTaiga(), provideHttpClient(), provideHttpClientTesting()],
        }).compileComponents();
        const fixture = TestBed.createComponent(Host);
        const http = TestBed.inject(HttpTestingController);
        fixture.detectChanges();
        http.expectOne('/api/meta/categories').flush([
            { category_id: 'C2', category_name: 'Minuman' },
            { category_id: 'C1', category_name: 'Makanan' },
        ]);
        http.expectOne('/api/meta/category-details').flush([
            { category_detail_id: 'D1', category_detail_name: 'Nasi', category_id: 'C1' },
            { category_detail_id: 'D2', category_detail_name: 'Kopi', category_id: 'C2' },
            { category_detail_id: 'D3', category_detail_name: 'Mie', category_id: 'C1' },
        ]);
        await fixture.whenStable();
        const el = fixture.nativeElement as HTMLElement;
        const flush = async () => { fixture.detectChanges(); await fixture.whenStable(); };
        const optionsOf = async (i: number) => {
            (el.querySelectorAll('.sel-trigger')[i] as HTMLButtonElement).click();
            await flush();
            const labels = [...el.querySelectorAll('.sel-opt')].map((o) => o.textContent?.trim());
            (el.querySelectorAll('.sel-trigger')[i] as HTMLButtonElement).click();
            await flush();
            return labels;
        };
        return { fixture, el, flush, optionsOf };
    };

    it('lists categories alphabetically and hides sub-categories until one is chosen', async () => {
        const { el, optionsOf } = await setup();
        expect(await optionsOf(0)).toEqual(['Semua kategori', 'Makanan', 'Minuman']);
        expect(el.querySelectorAll('.sel-trigger').length).toBe(1);
    });

    it('only offers sub-categories of the chosen category', async () => {
        const { fixture, flush, optionsOf } = await setup();
        fixture.componentInstance.category.set('C1');
        await flush();
        expect(await optionsOf(1)).toEqual(['Semua sub-kategori', 'Mie', 'Nasi']);
    });

    it('clears the sub-category when the category changes', async () => {
        const { fixture, flush } = await setup();
        fixture.componentInstance.category.set('C1');
        await flush();
        fixture.componentInstance.detail.set('D1');
        await flush();
        fixture.componentInstance.category.set('C2');
        await flush();
        expect(fixture.componentInstance.detail()).toBe('');
    });
});
