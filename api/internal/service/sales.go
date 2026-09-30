package service

import (
	"time"

	"gorm.io/gorm"
)

// nettSalesExpr is the current nett_sales formula (ported from
// supabase/migrations/20260821100000_nett_sales_formula_revision.sql):
// subtotal minus taxes, discounts and rounding, Finished sales only. Earlier
// formulas (grand_total-based, also subtracting voucher/order_fee/delivery)
// are superseded and intentionally not ported.
const nettSalesExpr = `subtotal - other_tax_total - vat_total - other_vat_total - discount_total - rounding_total`

type SalesSummary struct {
	Revenue       float64 `json:"revenue"`
	NettSales     float64 `json:"nett_sales"`
	TransCount    int64   `json:"trans_count"`
	MemberRevenue float64 `json:"member_revenue"`
}

// GetSalesSummary ports the MCP sales_summary tool.
func GetSalesSummary(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) (SalesSummary, error) {
	var s SalesSummary
	err := db.Raw(`
		select
			coalesce(sum(grand_total) filter (where status_name = 'Finished'), 0) as revenue,
			coalesce(sum(`+nettSalesExpr+`) filter (where status_name = 'Finished'), 0) as nett_sales,
			coalesce(count(*) filter (where status_name = 'Finished'), 0) as trans_count,
			coalesce(sum(grand_total) filter (where status_name = 'Finished' and member_code is not null), 0) as member_revenue
		from raw_sales
		where not is_non_sales
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
	`, dateStart, dateEnd, outlet, outlet).Scan(&s).Error
	return s, err
}

type DailyOutletMetric struct {
	SalesDate          time.Time `json:"sales_date"`
	BranchCode         string    `json:"branch_code"`
	Revenue            float64   `json:"revenue"`
	TransCount         int64     `json:"trans_count"`
	MemberRevenue      float64   `json:"member_revenue"`
	NonPromoRevenue    float64   `json:"non_promo_revenue"`
	NonPromoTransCount int64     `json:"non_promo_trans_count"`
	NettSales          float64   `json:"nett_sales"`
}

// GetSalesDaily ports v_sales_daily_outlet at full row granularity (per
// date+outlet), for trend charts and outlet leaderboards -- unlike
// sales_summary/revenue_by_outlet (MCP), which only return totals.
func GetSalesDaily(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) ([]DailyOutletMetric, error) {
	var rows []DailyOutletMetric
	err := db.Raw(`
		select
			sales_date,
			branch_code,
			coalesce(sum(grand_total) filter (where status_name = 'Finished'), 0) as revenue,
			coalesce(count(*) filter (where status_name = 'Finished'), 0) as trans_count,
			coalesce(sum(grand_total) filter (where status_name = 'Finished' and member_code is not null), 0) as member_revenue,
			coalesce(sum(grand_total) filter (where status_name = 'Finished' and (promotion_id is null or promotion_id = '0')), 0) as non_promo_revenue,
			coalesce(count(*) filter (where status_name = 'Finished' and (promotion_id is null or promotion_id = '0')), 0) as non_promo_trans_count,
			coalesce(sum(`+nettSalesExpr+`) filter (where status_name = 'Finished'), 0) as nett_sales
		from raw_sales
		where not is_non_sales
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		group by sales_date, branch_code
		order by sales_date, branch_code
	`, dateStart, dateEnd, outlet, outlet).Scan(&rows).Error
	return rows, err
}

type HourlyMetric struct {
	HourOfDay  int     `json:"hour_of_day"`
	Revenue    float64 `json:"revenue"`
	TransCount int64   `json:"trans_count"`
}

// GetSalesHourly ports v_sales_hourly_outlet, pre-aggregated across the
// whole date range into 24 hour buckets (peak-hour analysis is the only
// consumer) -- the old per-day-per-hour rows were summed client-side, which
// risked exceeding PostgREST's row cap for wide ranges; doing the SUM/GROUP
// BY in SQL avoids that entirely.
func GetSalesHourly(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) ([]HourlyMetric, error) {
	var rows []HourlyMetric
	err := db.Raw(`
		select
			extract(hour from sales_date_in)::int as hour_of_day,
			coalesce(sum(grand_total) filter (where status_name = 'Finished'), 0) as revenue,
			coalesce(count(*) filter (where status_name = 'Finished'), 0) as trans_count
		from raw_sales
		where not is_non_sales
			and sales_date_in is not null
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		group by extract(hour from sales_date_in)::int
		order by hour_of_day
	`, dateStart, dateEnd, outlet, outlet).Scan(&rows).Error
	return rows, err
}

type OutletRevenue struct {
	BranchCode string  `json:"branch_code"`
	BranchName string  `json:"branch_name"`
	Revenue    float64 `json:"revenue"`
	NettSales  float64 `json:"nett_sales"`
	TransCount int64   `json:"trans_count"`
}

// GetRevenueByOutlet ports the dashboard's outlet leaderboard: every outlet
// (including ones with zero revenue in range), with revenue+nett_sales+
// trans_count, sorted by revenue descending -- unlike the MCP
// revenue_by_outlet tool, which only returns revenue.
func GetRevenueByOutlet(db *gorm.DB, dateStart, dateEnd time.Time) ([]OutletRevenue, error) {
	var rows []OutletRevenue
	err := db.Raw(`
		select
			o.branch_code,
			o.branch_name,
			coalesce(sum(v.revenue), 0) as revenue,
			coalesce(sum(v.nett_sales), 0) as nett_sales,
			coalesce(sum(v.trans_count), 0) as trans_count
		from outlets o
		left join (
			select
				branch_code,
				sum(grand_total) filter (where status_name = 'Finished') as revenue,
				sum(`+nettSalesExpr+`) filter (where status_name = 'Finished') as nett_sales,
				count(*) filter (where status_name = 'Finished') as trans_count
			from raw_sales
			where not is_non_sales
				and sales_date between ? and ?
			group by branch_code
		) v on v.branch_code = o.branch_code
		group by o.branch_code, o.branch_name
		order by revenue desc
	`, dateStart, dateEnd).Scan(&rows).Error
	return rows, err
}

type CategoryRevenue struct {
	Revenue   float64 `json:"revenue"`
	NettSales float64 `json:"nett_sales"`
}

// GetRevenueByCategory ports v_sales_daily_outlet_category: nett_sales is
// allocated to each line item proportionally to its share of the bill's
// item total, then summed for the matching category/category-detail scope.
func GetRevenueByCategory(db *gorm.DB, dateStart, dateEnd time.Time, outlet, category, categoryDetail *string) (CategoryRevenue, error) {
	var r CategoryRevenue
	err := db.Raw(`
		with tx_nett as (
			select sales_num, `+nettSalesExpr+` as nett_sales
			from raw_sales
			where status_name = 'Finished' and not is_non_sales
				and sales_date between ? and ?
				and (?::text is null or branch_code = ?)
		),
		tx_item_total as (
			select sales_num, sum(total) as item_total
			from raw_sales_menu_items
			group by sales_num
		)
		select
			coalesce(sum(m.total), 0) as revenue,
			coalesce(sum(
				m.total * case when it.item_total <> 0 then n.nett_sales / it.item_total else 0 end
			), 0) as nett_sales
		from raw_sales_menu_items m
		join tx_nett n on n.sales_num = m.sales_num
		join tx_item_total it on it.sales_num = m.sales_num
		where (?::text is null or m.menu_category_id = ?)
			and (?::text is null or m.menu_category_detail_id = ?)
	`, dateStart, dateEnd, outlet, outlet, category, category, categoryDetail, categoryDetail).Scan(&r).Error
	return r, err
}

type Product struct {
	MenuID           string  `json:"menu_id"`
	MenuName         *string `json:"menu_name"`
	CategoryID       *string `json:"category_id"`
	Category         *string `json:"category"`
	CategoryDetailID *string `json:"category_detail_id"`
	CategoryDetail   *string `json:"category_detail"`
	Qty              float64 `json:"qty"`
	Revenue          float64 `json:"revenue"`
}

// GetTopProducts ports v_sales_product_daily, aggregated over the range.
// sortBy is "qty" or "revenue"; sortDesc controls direction -- the MCP
// top_products tool only sorts descending, but the dashboard needs ascending
// too (Slow Moving). limit <= 0 means unlimited (MCP always applies a limit;
// the dashboard's Top Seller/Slow Moving panels don't).
func GetTopProducts(db *gorm.DB, dateStart, dateEnd time.Time, outlet, category, categoryDetail *string, sortBy string, sortDesc bool, limit int) ([]Product, error) {
	if sortBy != "qty" && sortBy != "revenue" {
		sortBy = "revenue"
	}
	direction := "desc"
	if !sortDesc {
		direction = "asc"
	}

	query := `
		select
			menu_id, menu_name,
			menu_category_id as category_id, menu_category_name as category,
			menu_category_detail_id as category_detail_id, menu_category_detail_name as category_detail,
			coalesce(sum(qty), 0) as qty,
			coalesce(sum(total), 0) as revenue
		from raw_sales_menu_items m
		where sales_num in (select sales_num from raw_sales where status_name = 'Finished' and not is_non_sales)
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
			and (?::text is null or menu_category_id = ?)
			and (?::text is null or menu_category_detail_id = ?)
		group by menu_id, menu_name, menu_category_id, menu_category_name, menu_category_detail_id, menu_category_detail_name
		order by ` + sortBy + ` ` + direction

	args := []any{dateStart, dateEnd, outlet, outlet, category, category, categoryDetail, categoryDetail}
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}

	var rows []Product
	err := db.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

type MenuPerformance struct {
	MenuID             string   `json:"menu_id"`
	MenuName           *string  `json:"menu_name"`
	Category           *string  `json:"category"`
	CategoryDetail     *string  `json:"category_detail"`
	Qty                float64  `json:"qty"`
	Revenue            float64  `json:"revenue"`
	ContributionPct    *float64 `json:"contribution_pct"`
	Trend              string   `json:"trend"`
	IsTakeoutCandidate bool     `json:"is_takeout_candidate"`
}

// GetMenuPerformance ports fn_menu_performance exactly:
//   - previous period is the same length, ending the day before dateStart.
//   - contribution_pct is against the total revenue of every product in the
//     outlet scope, BEFORE the category/category-detail filter is applied
//     (so filtered percentages don't sum to 100 -- this is intentional,
//     matches the original FR-22 spec).
//   - trend: no previous data -> "stagnan"; qty > prev*1.1 -> "naik";
//     qty < prev*0.9 -> "turun"; otherwise "stagnan".
//   - is_takeout_candidate: qty < threshold (strictly less than).
//   - no LIMIT -- every menu in scope is returned, sorted by qty ascending.
func GetMenuPerformance(db *gorm.DB, dateStart, dateEnd time.Time, outlet, category, categoryDetail *string, threshold float64) ([]MenuPerformance, error) {
	var rows []MenuPerformance
	err := db.Raw(`
		with period_days as (
			select (?::date - ?::date + 1) as days
		),
		prev_range as (
			select
				(?::date - (select days from period_days)) as prev_start,
				(?::date - 1) as prev_end
		),
		product_line as (
			select
				m.sales_date, m.branch_code, m.menu_id, m.menu_name,
				m.menu_category_id as category_id, m.menu_category_name as category,
				m.menu_category_detail_id as category_detail_id, m.menu_category_detail_name as category_detail,
				m.qty, m.total as revenue
			from raw_sales_menu_items m
			join raw_sales s on s.sales_num = m.sales_num
			where s.status_name = 'Finished' and not s.is_non_sales
		),
		current_agg as (
			select
				menu_id, menu_name, category_id, category, category_detail_id, category_detail,
				sum(qty) as qty, sum(revenue) as revenue
			from product_line
			where sales_date between ? and ?
				and (?::text is null or branch_code = ?)
			group by menu_id, menu_name, category_id, category, category_detail_id, category_detail
		),
		previous_agg as (
			select menu_id, sum(qty) as qty
			from product_line, prev_range
			where sales_date between prev_range.prev_start and prev_range.prev_end
				and (?::text is null or branch_code = ?)
			group by menu_id
		),
		total_rev as (
			select sum(revenue) as total from current_agg
		)
		select
			c.menu_id, c.menu_name, c.category, c.category_detail, c.qty, c.revenue,
			round((c.revenue / nullif((select total from total_rev), 0) * 100)::numeric, 1) as contribution_pct,
			case
				when p.qty is null then 'stagnan'
				when c.qty > p.qty * 1.1 then 'naik'
				when c.qty < p.qty * 0.9 then 'turun'
				else 'stagnan'
			end as trend,
			(c.qty < ?) as is_takeout_candidate
		from current_agg c
		left join previous_agg p on p.menu_id = c.menu_id
		where (?::text is null or c.category_id = ?)
			and (?::text is null or c.category_detail_id = ?)
		order by c.qty asc
	`,
		dateEnd, dateStart,
		dateStart, dateStart,
		dateStart, dateEnd, outlet, outlet,
		outlet, outlet,
		threshold,
		category, category, categoryDetail, categoryDetail,
	).Scan(&rows).Error
	return rows, err
}

type Bill struct {
	BillNum    *string   `json:"bill_num"`
	SalesDate  time.Time `json:"sales_date"`
	BranchCode string    `json:"branch_code"`
	GrandTotal float64   `json:"grand_total"`
}

// GetSalesBills ports v_sales_bills with server-side pagination
// (page is 1-based).
func GetSalesBills(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string, page, pageSize int) ([]Bill, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	var total int64
	err := db.Raw(`
		select count(*) from raw_sales
		where status_name = 'Finished' and not is_non_sales
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
	`, dateStart, dateEnd, outlet, outlet).Scan(&total).Error
	if err != nil {
		return nil, 0, err
	}

	var rows []Bill
	err = db.Raw(`
		select bill_num, sales_date, branch_code, grand_total
		from raw_sales
		where status_name = 'Finished' and not is_non_sales
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		order by sales_date asc, bill_num asc
		limit ? offset ?
	`, dateStart, dateEnd, outlet, outlet, pageSize, (page-1)*pageSize).Scan(&rows).Error
	return rows, total, err
}

// GetAllSalesBills returns every matching bill with no pagination, for
// export -- there's no PostgREST row cap to work around in Go, so this is a
// single query rather than the old client-side chunked loop.
func GetAllSalesBills(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) ([]Bill, error) {
	var rows []Bill
	err := db.Raw(`
		select bill_num, sales_date, branch_code, grand_total
		from raw_sales
		where status_name = 'Finished' and not is_non_sales
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		order by sales_date asc, bill_num asc
	`, dateStart, dateEnd, outlet, outlet).Scan(&rows).Error
	return rows, err
}
