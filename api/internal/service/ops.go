package service

import (
	"time"

	"gorm.io/gorm"
)

type OpsTotals struct {
	TransCountAll        int64   `json:"trans_count_all"`
	TransCountFinished   int64   `json:"trans_count_finished"`
	CancelledCount       int64   `json:"cancelled_count"`
	VoidCount            int64   `json:"void_count"`
	NewCount             int64   `json:"new_count"`
	DwellSecondsSum      float64 `json:"dwell_seconds_sum"`
	DwellSampleCount     int64   `json:"dwell_sample_count"`
	PaxTotalSum          int64   `json:"pax_total_sum"`
	MenuDiscountSum      float64 `json:"menu_discount_sum"`
	PromotionDiscountSum float64 `json:"promotion_discount_sum"`
	VoucherDiscountSum   float64 `json:"voucher_discount_sum"`
}

type ChannelMetric struct {
	Channel    string  `json:"channel"`
	Revenue    float64 `json:"revenue"`
	TransCount int64   `json:"trans_count"`
}

type PaymentMethodMetric struct {
	PaymentMethodTypeName string  `json:"payment_method_type_name"`
	PaymentAmount         float64 `json:"payment_amount"`
	PaymentCount          int64   `json:"payment_count"`
}

type OpsSummary struct {
	Totals          OpsTotals             `json:"totals"`
	ByChannel       []ChannelMetric       `json:"by_channel"`
	ByPaymentMethod []PaymentMethodMetric `json:"by_payment_method"`
}

// GetOpsSummary ports the MCP ops_summary tool (v_sales_ops_daily,
// v_sales_channel_daily, v_sales_payment_method_daily), inlined as three
// direct queries against raw_sales/raw_sales_payments.
//
// Quirks preserved exactly:
//   - dwell time is capped at 8h (28800s) per bill, and only counted when
//     both sales_date_in/sales_date_out are set and out >= in.
//   - trans_count_all counts EVERY status, including the poorly-understood
//     'New' status -- it is not treated as an error/bad state.
//   - channel normalization collapses "dine in%"/"%pick up%"/"%delivery%"
//     case-insensitively; anything else (including null) passes through as
//     "Tidak Diketahui" for null, or the raw string otherwise.
//   - payment method breakdown is based on raw_sales_payments.payment_amount,
//     not grand_total, so split payments count once per payment row.
func GetOpsSummary(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) (OpsSummary, error) {
	var result OpsSummary

	err := db.Raw(`
		select
			coalesce(count(*), 0) as trans_count_all,
			coalesce(count(*) filter (where status_name = 'Finished'), 0) as trans_count_finished,
			coalesce(count(*) filter (where status_name = 'Cancelled'), 0) as cancelled_count,
			coalesce(count(*) filter (where status_name = 'Void'), 0) as void_count,
			coalesce(count(*) filter (where status_name = 'New'), 0) as new_count,
			coalesce(sum(least(extract(epoch from (sales_date_out - sales_date_in)), 28800))
				filter (where status_name = 'Finished' and sales_date_in is not null and sales_date_out is not null and sales_date_out >= sales_date_in), 0) as dwell_seconds_sum,
			coalesce(count(*)
				filter (where status_name = 'Finished' and sales_date_in is not null and sales_date_out is not null and sales_date_out >= sales_date_in), 0) as dwell_sample_count,
			coalesce(sum(pax_total) filter (where status_name = 'Finished' and pax_total is not null), 0) as pax_total_sum,
			coalesce(sum(menu_discount_total) filter (where status_name = 'Finished'), 0) as menu_discount_sum,
			coalesce(sum(promotion_discount) filter (where status_name = 'Finished'), 0) as promotion_discount_sum,
			coalesce(sum(voucher_discount_total) filter (where status_name = 'Finished'), 0) as voucher_discount_sum
		from raw_sales
		where sales_date between ? and ?
			and (?::text is null or branch_code = ?)
	`, dateStart, dateEnd, outlet, outlet).Scan(&result.Totals).Error
	if err != nil {
		return result, err
	}

	err = db.Raw(`
		select
			case
				when visit_purpose_name ilike 'dine in%' then 'Dine In'
				when visit_purpose_name ilike '%pick up%' then 'Pick Up'
				when visit_purpose_name ilike '%delivery%' then 'Delivery'
				when visit_purpose_name is null then 'Tidak Diketahui'
				else visit_purpose_name
			end as channel,
			coalesce(sum(grand_total), 0) as revenue,
			coalesce(count(*), 0) as trans_count
		from raw_sales
		where status_name = 'Finished'
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		group by channel
		order by revenue desc
	`, dateStart, dateEnd, outlet, outlet).Scan(&result.ByChannel).Error
	if err != nil {
		return result, err
	}

	err = db.Raw(`
		select
			coalesce(p.payment_method_type_name, 'Tidak Diketahui') as payment_method_type_name,
			coalesce(sum(p.payment_amount), 0) as payment_amount,
			coalesce(count(*), 0) as payment_count
		from raw_sales_payments p
		join raw_sales s on s.sales_num = p.sales_num
		where s.status_name = 'Finished'
			and s.sales_date between ? and ?
			and (?::text is null or s.branch_code = ?)
		group by p.payment_method_type_name
		order by payment_amount desc
	`, dateStart, dateEnd, outlet, outlet).Scan(&result.ByPaymentMethod).Error
	return result, err
}
