-- +goose Up
-- Raw tables: source of truth for data synced from the ESB OMS API (ETL job,
-- M3). Field names/shapes mirror the ESB `get-sales-information` response
-- body verbatim (bill-level fields flattened to snake_case; nested
-- salesPayments[]/salesMenus[] become child tables). Ported from the old
-- Supabase schema (supabase/migrations/20260813090100_raw_tables.sql) with
-- the raw_sales_menu_items primary key already in its final, corrected form
-- (sales_num, line_seq) -- see supabase/migrations/20260813100000_fix_menu_items_key.sql
-- for why menu_id+batch_id alone was never a reliable key.

create table outlets (
  branch_code text primary key,
  branch_name text not null,
  ext_branch_code text,
  first_seen_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table raw_sales (
  sales_num text primary key,
  bill_num text,
  sales_date date not null,
  sales_date_in timestamptz,
  sales_date_out timestamptz,
  branch_code text not null references outlets (branch_code),
  branch_name text,
  ext_branch_code text,
  -- Normalized to NULL when absent -- ESB sends "" for guest bills, so every
  -- query can rely on `member_code is not null` meaning "has a member".
  member_code text,
  member_name text,
  external_member_code text,
  table_id text,
  table_name text,
  visit_purpose_id text,
  visit_purpose_name text,
  visitor_type_id text,
  visitor_type_name text,
  pax_total int,
  subtotal numeric(14, 2),
  discount_total numeric(14, 2),
  menu_discount_total numeric(14, 2),
  promotion_discount numeric(14, 2),
  voucher_discount_total numeric(14, 2),
  other_tax_total numeric(14, 2),
  vat_total numeric(14, 2),
  other_vat_total numeric(14, 2),
  delivery_cost numeric(14, 2),
  order_fee numeric(14, 2),
  grand_total numeric(14, 2) not null default 0,
  voucher_total numeric(14, 2),
  rounding_total numeric(14, 2),
  payment_total numeric(14, 2),
  billing_print_count int,
  payment_print_count int,
  additional_info text,
  promotion_id text,
  promotion_name text,
  flag_inclusive text,
  status_id text,
  status_name text,
  full_name text,
  email text,
  phone_number text,
  created_by text,
  edited_by text,
  edited_date timestamptz,
  parent_link_sales_num text,
  child_link_sales_num jsonb,
  merge_table jsonb,
  raw jsonb not null,
  synced_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table raw_sales_payments (
  sales_num text not null references raw_sales (sales_num) on delete cascade,
  sales_payment_backend_id text not null,
  sales_payment_pos_id text,
  payment_method_type_id text,
  payment_method_type_name text,
  payment_method_id text,
  payment_method_name text,
  voucher_code text,
  card_number text,
  bank_name text,
  account_name text,
  payment_amount numeric(14, 2),
  full_payment_amount numeric(14, 2),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (sales_num, sales_payment_backend_id)
);

create table raw_sales_menu_items (
  sales_num text not null references raw_sales (sales_num) on delete cascade,
  -- Synthetic per-sale line sequence (the item's position in salesMenus[],
  -- assigned by the ETL) -- the only reliable key ESB provides is sales_num;
  -- individual line items have no natural unique key of their own.
  line_seq int not null,
  menu_id text not null,
  batch_id text,
  sales_date date not null,
  branch_code text not null,
  menu_category_id text,
  menu_category_name text,
  menu_category_detail_id text,
  menu_category_detail_name text,
  menu_name text,
  menu_code text,
  qty numeric(10, 2) not null default 0,
  original_price numeric(14, 2),
  price numeric(14, 2),
  discount numeric(14, 2),
  discount_value numeric(14, 2),
  other_tax_value numeric(14, 2),
  vat_value numeric(14, 2),
  total numeric(14, 2) not null default 0,
  notes text,
  status_id text,
  status_name text,
  promotion_detail_id text,
  menu_promotion_id text,
  sales_type text,
  -- Modifier-level detail (packages/extras) is never surfaced in the
  -- dashboard, so it's kept as raw JSON rather than normalized further.
  packages jsonb,
  extras jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (sales_num, line_seq)
);

-- Member profile dimension. Populated by a feature-flagged sync step (ETL,
-- M3) that may stay off until a real ESB membership endpoint is available --
-- tier/join_date stay null until then.
create table raw_members (
  member_code text primary key,
  member_name text,
  tier text,
  join_date date,
  home_branch_code text references outlets (branch_code),
  status text,
  external_member_code text,
  phone_number text,
  email text,
  raw jsonb,
  synced_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table sync_logs (
  id bigserial primary key,
  job_name text not null,
  target_date date,
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  status text not null default 'running' check (status in ('running', 'success', 'failed')),
  rows_synced int,
  error_message text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- +goose Down
drop table sync_logs;
drop table raw_members;
drop table raw_sales_menu_items;
drop table raw_sales_payments;
drop table raw_sales;
drop table outlets;
