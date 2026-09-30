package model

import "time"

// RawSale is one bill from the ESB OMS API `get-sales-information` response,
// flattened to snake_case. Raw holds the untouched source payload; typed
// columns are the fields the service layer (M2) actually queries on.
type RawSale struct {
	SalesNum             string `gorm:"primaryKey"`
	BillNum              *string
	SalesDate            time.Time `gorm:"not null"`
	SalesDateIn          *time.Time
	SalesDateOut         *time.Time
	BranchCode           string `gorm:"not null"`
	BranchName           *string
	ExtBranchCode        *string
	MemberCode           *string
	MemberName           *string
	ExternalMemberCode   *string
	TableID              *string
	TableName            *string
	VisitPurposeID       *string
	VisitPurposeName     *string
	VisitorTypeID        *string
	VisitorTypeName      *string
	PaxTotal             *int
	Subtotal             *float64 `gorm:"type:numeric(14,2)"`
	DiscountTotal        *float64 `gorm:"type:numeric(14,2)"`
	MenuDiscountTotal    *float64 `gorm:"type:numeric(14,2)"`
	PromotionDiscount    *float64 `gorm:"type:numeric(14,2)"`
	VoucherDiscountTotal *float64 `gorm:"type:numeric(14,2)"`
	OtherTaxTotal        *float64 `gorm:"type:numeric(14,2)"`
	VatTotal             *float64 `gorm:"type:numeric(14,2)"`
	OtherVatTotal        *float64 `gorm:"type:numeric(14,2)"`
	DeliveryCost         *float64 `gorm:"type:numeric(14,2)"`
	OrderFee             *float64 `gorm:"type:numeric(14,2)"`
	GrandTotal           float64  `gorm:"type:numeric(14,2);not null;default:0"`
	VoucherTotal         *float64 `gorm:"type:numeric(14,2)"`
	RoundingTotal        *float64 `gorm:"type:numeric(14,2)"`
	PaymentTotal         *float64 `gorm:"type:numeric(14,2)"`
	BillingPrintCount    *int
	PaymentPrintCount    *int
	AdditionalInfo       *string
	PromotionID          *string
	PromotionName        *string
	FlagInclusive        *string
	StatusID             *string
	StatusName           *string
	// IsNonSales: paid with payment method type 7 (see etl.NonSalesPaymentTypeID).
	// Such bills are excluded from every sales metric.
	IsNonSales         bool `gorm:"not null;default:false"`
	FullName           *string
	Email              *string
	PhoneNumber        *string
	CreatedBy          *string
	EditedBy           *string
	EditedDate         *time.Time
	ParentLinkSalesNum *string
	ChildLinkSalesNum  []byte `gorm:"type:jsonb"`
	MergeTable         []byte `gorm:"type:jsonb"`
	Raw                []byte `gorm:"type:jsonb;not null"`
	SyncedAt           time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// RawSalesPayment is one payment line for a RawSale (a bill can be split
// across multiple payment methods).
type RawSalesPayment struct {
	SalesNum              string `gorm:"primaryKey"`
	SalesPaymentBackendID string `gorm:"primaryKey"`
	SalesPaymentPosID     *string
	PaymentMethodTypeID   *string
	PaymentMethodTypeName *string
	PaymentMethodID       *string
	PaymentMethodName     *string
	VoucherCode           *string
	CardNumber            *string
	BankName              *string
	AccountName           *string
	PaymentAmount         *float64 `gorm:"type:numeric(14,2)"`
	FullPaymentAmount     *float64 `gorm:"type:numeric(14,2)"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// RawSalesMenuItem is one line item (menu/product) sold within a RawSale.
// LineSeq is the item's position in the source salesMenus[] array, assigned
// by the ETL job -- ESB provides no natural unique key for a line item.
type RawSalesMenuItem struct {
	SalesNum               string `gorm:"primaryKey"`
	LineSeq                int    `gorm:"primaryKey"`
	MenuID                 string `gorm:"not null"`
	BatchID                *string
	SalesDate              time.Time `gorm:"not null"`
	BranchCode             string    `gorm:"not null"`
	MenuCategoryID         *string
	MenuCategoryName       *string
	MenuCategoryDetailID   *string
	MenuCategoryDetailName *string
	MenuName               *string
	MenuCode               *string
	Qty                    float64  `gorm:"type:numeric(10,2);not null;default:0"`
	OriginalPrice          *float64 `gorm:"type:numeric(14,2)"`
	Price                  *float64 `gorm:"type:numeric(14,2)"`
	Discount               *float64 `gorm:"type:numeric(14,2)"`
	DiscountValue          *float64 `gorm:"type:numeric(14,2)"`
	OtherTaxValue          *float64 `gorm:"type:numeric(14,2)"`
	VatValue               *float64 `gorm:"type:numeric(14,2)"`
	Total                  float64  `gorm:"type:numeric(14,2);not null;default:0"`
	Notes                  *string
	StatusID               *string
	StatusName             *string
	PromotionDetailID      *string
	MenuPromotionID        *string
	SalesType              *string
	Packages               []byte `gorm:"type:jsonb"`
	Extras                 []byte `gorm:"type:jsonb"`
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
