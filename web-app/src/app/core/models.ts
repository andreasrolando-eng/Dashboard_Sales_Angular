// Response shapes of the Go API (api/internal/service). Field names are the
// snake_case JSON keys; dates arrive as ISO strings.

export interface Outlet { branch_code: string; branch_name: string }
export interface LastSync { job_name: string; finished_at: string | null; status: string; rows_synced: number | null }

export interface SalesSummary { revenue: number; nett_sales: number; trans_count: number; member_revenue: number }
export interface SalesDailyRow {
    sales_date: string; branch_code: string; revenue: number; trans_count: number; member_revenue: number;
    non_promo_revenue: number; non_promo_trans_count: number; nett_sales: number;
}
export interface HourlyRow { hour_of_day: number; revenue: number; trans_count: number }
export interface OutletRevenueRow { branch_code: string; branch_name: string; revenue: number; nett_sales: number; trans_count: number }
export interface TopProduct {
    menu_id: string; menu_name: string; category_id: string; category: string;
    category_detail_id: string; category_detail: string; qty: number; revenue: number;
}
export interface MenuPerformanceRow {
    menu_id: string; menu_name: string; category: string; category_detail: string; qty: number; revenue: number;
    contribution_pct: number; trend: string; is_takeout_candidate: boolean;
}
export interface Bill { bill_num: string; sales_date: string; branch_code: string; grand_total: number }
export interface BillsPage { rows: Bill[]; total_count: number }

export interface OpsSummary {
    totals: {
        trans_count_all: number; trans_count_finished: number; cancelled_count: number; void_count: number;
        new_count: number; dwell_seconds_sum: number; dwell_sample_count: number; pax_total_sum: number;
        menu_discount_sum: number; promotion_discount_sum: number; voucher_discount_sum: number;
    };
    by_channel: { channel: string; revenue: number; trans_count: number }[];
    by_payment_method: { payment_method_type_name: string; payment_amount: number; payment_count: number }[];
}

export interface MembershipSummary {
    total_members: number; active_members: number; active_pct: number; churn_pct: number;
    retention_pct: number | null; visit_frequency: number;
}
export interface TopMember {
    member_code: string; member_name: string; outlet_name: string; tier: string;
    visits: number; spending: number; favorite_menu: string;
}
export interface NewMembersWeek { week_start: string; new_members: number }

export interface PromoRow {
    promotion_id: string; promotion_name: string; redemptions: number; promo_revenue: number;
    discount_cost: number; lift_pct: number | null; roi: number | null; status: string;
}

export interface AdminUser { id: number; email: string; name: string; is_admin: boolean; created_at: string }

// ---- Manual sync (/api/admin/sync) ----
export interface RowCounts { outlets: number; sales: number; payments: number; items: number }
export interface BillRef { sales_num: string; bill_num: string; branch_code: string; status: string; grand_total: number }
export interface StatusChange extends BillRef { from: string; to: string }
export type SyncMode = 'fill' | 'refresh';
export interface DaySyncResult {
    date: string; ok: boolean; records_fetched?: number;
    /** Rows newly written (manual sync = inserted only). */
    outlets?: number; sales?: number; payments?: number; menu_items?: number;
    /** Everything ESB returned for the day, and the part that was already stored. */
    found: RowCounts; existing?: RowCounts; error?: string;
    /** Found bills split into not-stored-before vs already-stored (skipped by fill, overwritten by refresh). */
    sales_new: number; sales_refreshed: number;
    /** Refresh only: bills whose status differs from what was stored, and stored bills ESB no longer returns. */
    status_changes?: StatusChange[]; not_in_esb?: BillRef[];
}
export type SyncStatus = 'pending' | 'running' | 'done' | 'failed';
export interface ManualDay { date: string; status: SyncStatus; result?: DaySyncResult }
export interface ManualJob {
    id: string; mode: SyncMode; status: 'running' | 'done' | 'failed'; date_from: string; date_to: string;
    days: ManualDay[]; started_at: string; finished_at?: string; error?: string;
}
export interface SyncLog {
    id: number; job_name: string; target_date: string | null; started_at: string; finished_at: string | null;
    status: string; rows_synced: number | null; error_message: string | null;
}
