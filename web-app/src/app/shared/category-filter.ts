import { Component, computed, effect, model, untracked } from '@angular/core';
import { httpResource } from '@angular/common/http';
import { CategoryDetailOption, CategoryOption } from '../core/models';
import { Select, SelectOption } from './select';

/**
 * Menu category + sub-category pickers for a panel header. The sub-category
 * list only shows details of the chosen category, and is cleared when the
 * category changes so the two never contradict each other.
 */
@Component({
    selector: 'app-category-filter',
    imports: [Select],
    template: `
        <div class="cat-filter">
            <app-select [options]="categoryOptions()" [(value)]="category" ariaLabel="Kategori menu" />
            @if (category()) {
                <app-select [options]="detailOptions()" [(value)]="detail" ariaLabel="Sub-kategori menu" />
            }
        </div>
    `,
})
export class CategoryFilter {
    readonly category = model('');
    readonly detail = model('');

    private readonly categories = httpResource<CategoryOption[]>(() => '/api/meta/categories');
    private readonly details = httpResource<CategoryDetailOption[]>(() => '/api/meta/category-details');

    constructor() {
        effect(() => {
            this.category();
            untracked(() => this.detail.set(''));
        });
    }

    protected readonly categoryOptions = computed<SelectOption[]>(() => [
        { value: '', label: 'Semua kategori' },
        ...byLabel((this.categories.value() ?? []).map((c) => ({ value: c.category_id, label: c.category_name }))),
    ]);

    protected readonly detailOptions = computed<SelectOption[]>(() => [
        { value: '', label: 'Semua sub-kategori' },
        ...byLabel(
            (this.details.value() ?? [])
                .filter((d) => d.category_id === this.category())
                .map((d) => ({ value: d.category_detail_id, label: d.category_detail_name })),
        ),
    ]);
}

function byLabel(options: SelectOption[]): SelectOption[] {
    return options.sort((a, b) => a.label.localeCompare(b.label, 'id'));
}
