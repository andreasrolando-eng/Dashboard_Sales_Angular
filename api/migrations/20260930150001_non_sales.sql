-- +goose Up
-- A bill is "non sales" when it is paid with payment method TYPE 7. ESB does not
-- allow type 7 to be combined with other types, so the whole bill is non sales.
-- Non sales bills are excluded from every sales metric and reported on their own.
-- The flag is set by the ETL from the bill's payments; this backfills old rows.
alter table raw_sales add column is_non_sales boolean not null default false;

update raw_sales s set is_non_sales = true
where exists (
  select 1 from raw_sales_payments p
  where p.sales_num = s.sales_num and p.payment_method_type_id = '7'
);

create index raw_sales_non_sales_date_idx on raw_sales (sales_date) where is_non_sales;

-- +goose Down
drop index raw_sales_non_sales_date_idx;
alter table raw_sales drop column is_non_sales;
