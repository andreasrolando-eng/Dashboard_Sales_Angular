package service

import (
	"time"

	"gorm.io/gorm"
)

type PromoPerformance struct {
	PromotionID   string   `json:"promotion_id"`
	PromotionName *string  `json:"promotion_name"`
	Redemptions   int64    `json:"redemptions"`
	PromoRevenue  float64  `json:"promo_revenue"`
	DiscountCost  float64  `json:"discount_cost"`
	LiftPct       *float64 `json:"lift_pct"`
	ROI           float64  `json:"roi"`
	Status        string   `json:"status"`
}

// GetPromoPerformance ports fn_promo_performance + v_promo_daily exactly:
//   - "promo" means promotion_id is not null and not the '0' sentinel.
//   - ONE shared baseline (average grand_total of non-promo Finished bills in
//     the same range/outlet scope) is used for every promo row.
//   - discount_cost is the WHOLE bill's discount_total, not scoped to the
//     promotion's own discount.
//   - lift_pct = (avg promo bill - baseline avg) / baseline avg * 100, nil
//     when the baseline is 0. roi defaults to 0 (never nil) when
//     discount_cost is 0.
//   - status is 'Efektif' only when lift >= 15 AND roi >= 2, using the
//     UNROUNDED values for that check (matches the original, which computes
//     the condition separately from the rounded output columns).
//   - grouped by (promotion_id, promotion_name), so a renamed promo splits
//     into separate rows. No LIMIT; ordered by redemptions descending.
func GetPromoPerformance(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) ([]PromoPerformance, error) {
	var rows []PromoPerformance
	err := db.Raw(`
		with baseline as (
			select
				coalesce(sum(grand_total) filter (where promotion_id is null or promotion_id = '0'), 0) as revenue,
				coalesce(count(*) filter (where promotion_id is null or promotion_id = '0'), 0) as trans_count
			from raw_sales
			where status_name = 'Finished' and not is_non_sales
				and sales_date between ? and ?
				and (?::text is null or branch_code = ?)
		),
		baseline_avg as (
			select case when trans_count > 0 then revenue / trans_count else 0 end as avg_grand_total
			from baseline
		),
		promo_agg as (
			select
				promotion_id, promotion_name,
				count(*) as redemptions,
				sum(grand_total) as promo_revenue,
				sum(discount_total) as discount_cost
			from raw_sales
			where status_name = 'Finished' and not is_non_sales
				and promotion_id is not null and promotion_id <> '0'
				and sales_date between ? and ?
				and (?::text is null or branch_code = ?)
			group by promotion_id, promotion_name
		),
		calc as (
			select
				p.promotion_id, p.promotion_name, p.redemptions, p.promo_revenue, p.discount_cost,
				case when p.redemptions > 0 then p.promo_revenue / p.redemptions else 0 end as avg_promo_bill,
				ba.avg_grand_total as baseline_avg
			from promo_agg p, baseline_avg ba
		)
		select
			promotion_id, promotion_name, redemptions, promo_revenue, discount_cost,
			round(((avg_promo_bill - baseline_avg) / nullif(baseline_avg, 0) * 100)::numeric, 1) as lift_pct,
			round(coalesce((promo_revenue - redemptions * baseline_avg) / nullif(discount_cost, 0), 0)::numeric, 2) as roi,
			case
				when (avg_promo_bill - baseline_avg) / nullif(baseline_avg, 0) * 100 >= 15
					and coalesce((promo_revenue - redemptions * baseline_avg) / nullif(discount_cost, 0), 0) >= 2
				then 'Efektif'
				else 'Kurang Efektif'
			end as status
		from calc
		order by redemptions desc
	`,
		dateStart, dateEnd, outlet, outlet, // baseline
		dateStart, dateEnd, outlet, outlet, // promo_agg
	).Scan(&rows).Error
	return rows, err
}
