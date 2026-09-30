package etl

import (
	"gorm.io/gorm"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// BillRef identifies a stored bill in sync reports.
type BillRef struct {
	SalesNum   string  `json:"sales_num"`
	BillNum    string  `json:"bill_num"`
	BranchCode string  `json:"branch_code"`
	Status     string  `json:"status"`
	GrandTotal float64 `json:"grand_total"`
}

// StatusChange is a stored bill whose status differs from what ESB returns now
// (typically Finished -> Void after a same-day-after void).
type StatusChange struct {
	BillRef
	From string `json:"from"`
	To   string `json:"to"`
}

// Comparison describes how one day's fetched bills relate to what is stored,
// computed BEFORE the write so a refresh can say exactly what it changed.
type Comparison struct {
	New           int            // fetched bills that are not stored yet
	Refreshed     int            // fetched bills that are stored and will be overwritten
	StatusChanges []StatusChange // subset of Refreshed whose status differs
	// NotInESB are bills stored for this date that ESB no longer returns.
	// They are only reported, never modified or deleted: absence from the
	// response is not proof of a void.
	NotInESB []BillRef
}

// CompareWithStored diffs the fetched sales of `date` against raw_sales.
func CompareWithStored(db *gorm.DB, date string, incoming []model.RawSale) (Comparison, error) {
	nums := make([]string, 0, len(incoming))
	for _, s := range incoming {
		nums = append(nums, s.SalesNum)
	}

	var stored []model.RawSale
	if err := db.Select("sales_num", "bill_num", "branch_code", "status_name", "grand_total").
		Where("sales_date = ? OR sales_num IN ?", date, nums).
		Find(&stored).Error; err != nil {
		return Comparison{}, err
	}
	byNum := make(map[string]model.RawSale, len(stored))
	for _, s := range stored {
		byNum[s.SalesNum] = s
	}

	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	ref := func(s model.RawSale) BillRef {
		return BillRef{SalesNum: s.SalesNum, BillNum: str(s.BillNum), BranchCode: s.BranchCode, Status: str(s.StatusName), GrandTotal: s.GrandTotal}
	}

	var cmp Comparison
	fetched := make(map[string]bool, len(incoming))
	for _, in := range incoming {
		fetched[in.SalesNum] = true
		old, exists := byNum[in.SalesNum]
		if !exists {
			cmp.New++
			continue
		}
		cmp.Refreshed++
		if str(old.StatusName) != str(in.StatusName) {
			r := ref(old)
			r.GrandTotal = in.GrandTotal
			cmp.StatusChanges = append(cmp.StatusChanges, StatusChange{BillRef: r, From: str(old.StatusName), To: str(in.StatusName)})
		}
	}
	// Rows matched only by sales_num are always in `fetched`, so every stored
	// row that is not fetched here came from the sales_date filter.
	for _, s := range stored {
		if !fetched[s.SalesNum] {
			cmp.NotInESB = append(cmp.NotInESB, ref(s))
		}
	}
	return cmp, nil
}
