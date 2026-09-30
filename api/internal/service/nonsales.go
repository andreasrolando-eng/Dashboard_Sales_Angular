package service

import (
	"time"

	"gorm.io/gorm"
)

// Non sales = Finished bills paid with payment method type 7 (raw_sales.is_non_sales,
// set by the ETL). They are excluded from every sales metric in this package and
// reported only here. "Nilai" is the bill's grand_total.

type NonSalesSummary struct {
	Value       float64 `json:"value"`
	TransCount  int64   `json:"trans_count"`
	AvgValue    float64 `json:"avg_value"`
	OutletCount int64   `json:"outlet_count"`
}

func GetNonSalesSummary(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) (NonSalesSummary, error) {
	var s NonSalesSummary
	err := db.Raw(`
		select
			coalesce(sum(grand_total), 0) as value,
			count(*) as trans_count,
			coalesce(avg(grand_total), 0) as avg_value,
			count(distinct branch_code) as outlet_count
		from raw_sales
		where is_non_sales and status_name = 'Finished'
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
	`, dateStart, dateEnd, outlet, outlet).Scan(&s).Error
	return s, err
}

type NonSalesDaily struct {
	SalesDate  time.Time `json:"sales_date"`
	Value      float64   `json:"value"`
	TransCount int64     `json:"trans_count"`
}

func GetNonSalesDaily(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string) ([]NonSalesDaily, error) {
	rows := []NonSalesDaily{}
	err := db.Raw(`
		select sales_date, coalesce(sum(grand_total), 0) as value, count(*) as trans_count
		from raw_sales
		where is_non_sales and status_name = 'Finished'
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
		group by sales_date
		order by sales_date
	`, dateStart, dateEnd, outlet, outlet).Scan(&rows).Error
	return rows, err
}

type NonSalesOutlet struct {
	BranchCode string  `json:"branch_code"`
	BranchName string  `json:"branch_name"`
	Value      float64 `json:"value"`
	TransCount int64   `json:"trans_count"`
}

// GetNonSalesByOutlet lists only outlets that have non sales in the range
// (unlike the sales leaderboard, zero rows would just be noise here).
func GetNonSalesByOutlet(db *gorm.DB, dateStart, dateEnd time.Time) ([]NonSalesOutlet, error) {
	rows := []NonSalesOutlet{}
	err := db.Raw(`
		select s.branch_code, coalesce(o.branch_name, s.branch_code) as branch_name,
			sum(s.grand_total) as value, count(*) as trans_count
		from raw_sales s
		left join outlets o on o.branch_code = s.branch_code
		where s.is_non_sales and s.status_name = 'Finished'
			and s.sales_date between ? and ?
		group by s.branch_code, o.branch_name
		order by value desc
	`, dateStart, dateEnd).Scan(&rows).Error
	return rows, err
}

type NonSalesMenu struct {
	MenuID   string  `json:"menu_id"`
	MenuName *string `json:"menu_name"`
	Category *string `json:"category"`
	Qty      float64 `json:"qty"`
	Value    float64 `json:"value"`
}

// GetNonSalesTopMenus: which menus go out as non sales, by quantity.
func GetNonSalesTopMenus(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string, limit int) ([]NonSalesMenu, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows := []NonSalesMenu{}
	err := db.Raw(`
		select m.menu_id, m.menu_name, m.menu_category_name as category,
			coalesce(sum(m.qty), 0) as qty, coalesce(sum(m.total), 0) as value
		from raw_sales_menu_items m
		join raw_sales s on s.sales_num = m.sales_num
		where s.is_non_sales and s.status_name = 'Finished'
			and s.sales_date between ? and ?
			and (?::text is null or s.branch_code = ?)
		group by m.menu_id, m.menu_name, m.menu_category_name
		order by qty desc, value desc
		limit ?
	`, dateStart, dateEnd, outlet, outlet, limit).Scan(&rows).Error
	return rows, err
}

type NonSalesBill struct {
	BillNum       *string   `json:"bill_num"`
	SalesDate     time.Time `json:"sales_date"`
	BranchCode    string    `json:"branch_code"`
	GrandTotal    float64   `json:"grand_total"`
	PaymentMethod *string   `json:"payment_method"`
	MemberName    *string   `json:"member_name"`
}

// GetNonSalesBills is paginated (page is 1-based). payment_method lists the
// type-7 payment method names used on the bill (e.g. "Compliment").
func GetNonSalesBills(db *gorm.DB, dateStart, dateEnd time.Time, outlet *string, page, pageSize int) ([]NonSalesBill, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	var total int64
	if err := db.Raw(`
		select count(*) from raw_sales
		where is_non_sales and status_name = 'Finished'
			and sales_date between ? and ?
			and (?::text is null or branch_code = ?)
	`, dateStart, dateEnd, outlet, outlet).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []NonSalesBill{}
	err := db.Raw(`
		select s.bill_num, s.sales_date, s.branch_code, s.grand_total, s.member_name,
			(select string_agg(distinct coalesce(p.payment_method_name, p.payment_method_type_name), ', ')
				from raw_sales_payments p where p.sales_num = s.sales_num) as payment_method
		from raw_sales s
		where s.is_non_sales and s.status_name = 'Finished'
			and s.sales_date between ? and ?
			and (?::text is null or s.branch_code = ?)
		order by s.sales_date desc, s.bill_num
		limit ? offset ?
	`, dateStart, dateEnd, outlet, outlet, pageSize, (page-1)*pageSize).Scan(&rows).Error
	return rows, total, err
}
