-- +goose Up
-- Keeps future aggregation queries (internal/service, M2) fast as raw_sales
-- grows. Ported from supabase/migrations/20260813090200_indexes.sql.
create index idx_raw_sales_date_branch on raw_sales (sales_date, branch_code);
create index idx_raw_sales_member on raw_sales (member_code) where member_code is not null;
create index idx_raw_sales_promotion on raw_sales (promotion_id) where promotion_id is not null and promotion_id <> '0';
create index idx_raw_sales_status on raw_sales (status_name);

create index idx_raw_sales_menu_items_date_branch on raw_sales_menu_items (sales_date, branch_code);
create index idx_raw_sales_menu_items_category on raw_sales_menu_items (menu_category_name);
-- No separate index on sales_num alone: the primary key (sales_num, line_seq)
-- already serves sales_num-prefix lookups (e.g. the FK join back to raw_sales).

create index idx_sync_logs_job_started on sync_logs (job_name, started_at desc);

-- +goose Down
drop index idx_sync_logs_job_started;
drop index idx_raw_sales_menu_items_category;
drop index idx_raw_sales_menu_items_date_branch;
drop index idx_raw_sales_status;
drop index idx_raw_sales_promotion;
drop index idx_raw_sales_member;
drop index idx_raw_sales_date_branch;
