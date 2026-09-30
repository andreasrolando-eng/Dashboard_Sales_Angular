package service

import (
	"time"

	"gorm.io/gorm"
)

// membersDimCTE ports v_member_visits_daily + v_member_branch_counts +
// v_members_dim as a reusable CTE fragment (no trailing comma -- callers
// that add more CTEs after it append ",\n" themselves; callers that don't
// go straight to "select ... from members_dim").
//
// Members are DERIVED from raw_sales, not read from raw_members directly:
// a member only exists here if they have at least one Finished sale with a
// member_code. raw_members only supplies tier/name overrides. Home branch is
// whichever branch has the most visits (ties broken arbitrarily). Tier
// falls back to a lifetime-spending bracket when raw_members.tier is unset.
const membersDimCTE = `
member_visits_daily as (
	select sales_date, member_code, branch_code, count(*) as visit_count, sum(grand_total) as spending
	from raw_sales
	where status_name = 'Finished' and not is_non_sales and member_code is not null
	group by sales_date, member_code, branch_code
),
member_branch_counts as (
	select member_code, branch_code, sum(visit_count) as visits
	from member_visits_daily
	group by member_code, branch_code
),
members_dim as (
	with lifetime as (
		select
			member_code,
			min(sales_date) as first_seen_date,
			max(sales_date) as last_seen_date,
			sum(visit_count) as total_visits,
			sum(spending) as total_spending
		from member_visits_daily
		group by member_code
	),
	home as (
		select distinct on (member_code) member_code, branch_code as home_branch_code
		from member_branch_counts
		order by member_code, visits desc
	),
	latest_name as (
		select distinct on (member_code) member_code, member_name
		from raw_sales
		where member_code is not null and member_name is not null
		order by member_code, sales_date desc
	)
	select
		l.member_code,
		coalesce(rm.member_name, ln.member_name) as member_name,
		home.home_branch_code,
		o.branch_name as home_branch_name,
		l.first_seen_date, l.last_seen_date, l.total_visits, l.total_spending,
		coalesce(
			rm.tier,
			case
				when l.total_spending >= 5000000 then 'Gold'
				when l.total_spending >= 2000000 then 'Silver'
				else 'Bronze'
			end
		) as tier
	from lifetime l
	left join home on home.member_code = l.member_code
	left join outlets o on o.branch_code = home.home_branch_code
	left join raw_members rm on rm.member_code = l.member_code
	left join latest_name ln on ln.member_code = l.member_code
)`

type MembershipSummary struct {
	TotalMembers   int64    `json:"total_members"`
	ActiveMembers  int64    `json:"active_members"`
	ActivePct      *float64 `json:"active_pct"`
	ChurnPct       *float64 `json:"churn_pct"`
	RetentionPct   *float64 `json:"retention_pct"`
	VisitFrequency *float64 `json:"visit_frequency"`
}

// GetMembershipSummary ports fn_membership_summary exactly:
//   - total_members/active_members are LIFETIME counts, scoped only by home
//     branch, ignoring dateStart entirely. "Active" means the member's
//     lifetime last_seen_date >= dateEnd-60 days.
//   - retention splits [dateStart, dateEnd] at its midpoint; it's the share
//     of first-half members who also visited in the second half, scoped by
//     the branch where the VISIT happened (not home branch) -- so totals and
//     retention use different outlet scoping, exactly as the original does.
//   - visit_frequency is average Finished-bill visits per member who visited
//     in the period, divided by max(days/30, 1) months.
//   - every percentage is nil (SQL NULL) when its denominator is 0, never 0.
func GetMembershipSummary(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) (MembershipSummary, error) {
	var s MembershipSummary
	err := db.Raw(`
		with `+membersDimCTE+`,
		totals as (
			select
				count(*) as total,
				count(*) filter (where last_seen_date >= ?::date - 60) as active
			from members_dim
			where (?::text is null or home_branch_code = ?)
		),
		mid as (
			select ?::date + floor((?::date - ?::date) / 2.0)::int as mid_date
		),
		first_half as (
			select distinct v.member_code
			from member_visits_daily v, mid
			where v.sales_date between ? and mid.mid_date
				and (?::text is null or v.branch_code = ?)
		),
		second_half as (
			select distinct v.member_code
			from member_visits_daily v, mid
			where v.sales_date > mid.mid_date and v.sales_date <= ?
				and (?::text is null or v.branch_code = ?)
		),
		retained as (
			select count(*) as n
			from first_half f
			where exists (select 1 from second_half s where s.member_code = f.member_code)
		),
		period_visits as (
			select v.member_code, sum(v.visit_count) as visits
			from member_visits_daily v
			where v.sales_date between ? and ?
				and (?::text is null or v.branch_code = ?)
			group by v.member_code
		),
		span as (
			select greatest((?::date - ?::date + 1) / 30.0, 1.0) as months
		)
		select
			totals.total as total_members,
			totals.active as active_members,
			round(totals.active::numeric / nullif(totals.total, 0) * 100, 1) as active_pct,
			round(100 - (totals.active::numeric / nullif(totals.total, 0) * 100), 1) as churn_pct,
			round(retained.n::numeric / nullif((select count(*) from first_half), 0) * 100, 1) as retention_pct,
			round(coalesce((select avg(visits) from period_visits), 0) / (select months from span), 1) as visit_frequency
		from totals, retained
	`,
		dateEnd,        // totals: active cutoff
		outlet, outlet, // totals: home branch filter
		dateStart, dateEnd, dateStart, // mid = dateStart + floor((dateEnd-dateStart)/2)
		dateStart,      // first_half: range start
		outlet, outlet, // first_half: outlet filter
		dateEnd,        // second_half: range end
		outlet, outlet, // second_half: outlet filter
		dateStart, dateEnd, // period_visits: range
		outlet, outlet, // period_visits: outlet filter
		dateEnd, dateStart, // span: dateEnd - dateStart + 1
	).Scan(&s).Error
	return s, err
}

type TopMember struct {
	MemberCode   string  `json:"member_code"`
	MemberName   *string `json:"member_name"`
	OutletName   *string `json:"outlet_name"`
	Tier         *string `json:"tier"`
	Visits       int64   `json:"visits"`
	Spending     float64 `json:"spending"`
	FavoriteMenu *string `json:"favorite_menu"`
}

// GetTopMembers ports fn_top_members, WITH member_name included (unlike the
// MCP mcp_top_members wrapper, which deliberately strips it for data
// minimization -- this is the internal dashboard API, not the public MCP
// surface). outlet_name is the member's home branch, which can differ from
// the outlet filter. tier is lifetime-based, not period-based. favorite_menu
// ties break by menu_name ascending.
func GetTopMembers(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string, limit int) ([]TopMember, error) {
	if limit <= 0 {
		limit = 8
	}
	var rows []TopMember
	err := db.Raw(`
		with `+membersDimCTE+`,
		agg as (
			select v.member_code, sum(v.visit_count) as visits, sum(v.spending) as spending
			from member_visits_daily v
			where v.sales_date between ? and ?
				and (?::text is null or v.branch_code = ?)
			group by v.member_code
		),
		member_menu_daily as (
			select s.member_code, m.sales_date, m.branch_code, m.menu_id, m.menu_name, sum(m.qty) as qty
			from raw_sales_menu_items m
			join raw_sales s on s.sales_num = m.sales_num
			where s.status_name = 'Finished' and not s.is_non_sales and s.member_code is not null
			group by s.member_code, m.sales_date, m.branch_code, m.menu_id, m.menu_name
		),
		menu_agg as (
			select v.member_code, v.menu_name, sum(v.qty) as qty
			from member_menu_daily v
			where v.sales_date between ? and ?
				and (?::text is null or v.branch_code = ?)
			group by v.member_code, v.menu_name
		),
		favorite as (
			select distinct on (member_code) member_code, menu_name as favorite_menu
			from menu_agg
			order by member_code, qty desc, menu_name
		)
		select
			a.member_code, d.member_name, d.home_branch_name as outlet_name, d.tier,
			a.visits, a.spending, f.favorite_menu
		from agg a
		join members_dim d on d.member_code = a.member_code
		left join favorite f on f.member_code = a.member_code
		order by a.spending desc
		limit ?
	`,
		dateStart, dateEnd, outlet, outlet, // agg
		dateStart, dateEnd, outlet, outlet, // menu_agg
		limit,
	).Scan(&rows).Error
	return rows, err
}

type MemberOption struct {
	MemberCode string  `json:"member_code"`
	MemberName *string `json:"member_name"`
}

// GetMemberOptions lists every derived member, for a member picker --
// no date range, only an optional home-branch filter.
func GetMemberOptions(db *gorm.DB, outlet *string) ([]MemberOption, error) {
	var rows []MemberOption
	err := db.Raw(`
		with `+membersDimCTE+`
		select member_code, member_name
		from members_dim
		where (?::text is null or home_branch_code = ?)
		order by member_name
	`,
		outlet, outlet,
	).Scan(&rows).Error
	return rows, err
}

type MemberMenuPurchase struct {
	MenuID           string    `json:"menu_id"`
	MenuName         *string   `json:"menu_name"`
	Qty              float64   `json:"qty"`
	Revenue          float64   `json:"revenue"`
	LastPurchaseDate time.Time `json:"last_purchase_date"`
}

// GetMemberMenuPurchases ports fn_member_menu_purchases: what one member
// bought in range, ordered by qty descending.
func GetMemberMenuPurchases(db *gorm.DB, memberCode string, dateStart, dateEnd time.Time, outlet *string) ([]MemberMenuPurchase, error) {
	var rows []MemberMenuPurchase
	err := db.Raw(`
		with member_menu_daily as (
			select s.member_code, m.sales_date, m.branch_code, m.menu_id, m.menu_name, m.qty, m.total as revenue
			from raw_sales_menu_items m
			join raw_sales s on s.sales_num = m.sales_num
			where s.status_name = 'Finished' and not s.is_non_sales and s.member_code is not null
		)
		select menu_id, menu_name, sum(qty) as qty, sum(revenue) as revenue, max(sales_date) as last_purchase_date
		from member_menu_daily
		where member_code = ?
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		group by menu_id, menu_name
		order by qty desc
	`,
		memberCode, dateStart, dateEnd, outlet, outlet,
	).Scan(&rows).Error
	return rows, err
}

type WeeklyNewMembers struct {
	WeekStart  time.Time `json:"week_start"`
	NewMembers int64     `json:"new_members"`
}

// GetMembershipNewWeekly ports v_membership_new_weekly, pre-windowed to the
// 8 weeks ending dateEnd (done in SQL now, not by fetching everything and
// slicing client-side like the old dashboard did).
func GetMembershipNewWeekly(db *gorm.DB, dateEnd time.Time) ([]WeeklyNewMembers, error) {
	var rows []WeeklyNewMembers
	err := db.Raw(`
		with `+membersDimCTE+`
		select date_trunc('week', first_seen_date)::date as week_start, count(*) as new_members
		from members_dim
		where date_trunc('week', first_seen_date)::date >= ?::date - 56
			and date_trunc('week', first_seen_date)::date <= ?::date
		group by week_start
		order by week_start
	`,
		dateEnd, dateEnd,
	).Scan(&rows).Error
	return rows, err
}
