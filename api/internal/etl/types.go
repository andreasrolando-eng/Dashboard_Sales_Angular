// Package etl ports the old Supabase Edge Function (supabase/functions/sync-esb)
// that pulls sales data from the ESB OMS API into the raw_* tables. Field
// names below mirror ESB's `get-sales-information` response verbatim -- see
// the port notes in each file for exact behavior parity with the original.
package etl

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// flexNumber accepts a JSON number OR a numeric string (ESB sends paxTotal
// as either depending on the record) and always unmarshals to a float64.
type flexNumber struct {
	Value *float64
}

func (f *flexNumber) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		f.Value = nil
		return nil
	}
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		f.Value = &num
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("flexNumber: %w", err)
	}
	if s == "" {
		f.Value = nil
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flexNumber: parse %q: %w", s, err)
	}
	f.Value = &n
	return nil
}

// MarshalJSON serializes back to a plain JSON number (or null) -- without
// this, encoding/json falls back to marshaling the struct's exported Value
// field as a JSON object, which corrupts the `raw` jsonb backup copy
// (Transform round-trips the whole decoded record through mustMarshal) and
// breaks any test/tooling that re-encodes an esbSaleRecord.
func (f flexNumber) MarshalJSON() ([]byte, error) {
	if f.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*f.Value)
}

// esbSaleRecord is one bill from ESB's sales-information response.
type esbSaleRecord struct {
	SalesNum     string  `json:"salesNum"`
	BillNum      *string `json:"billNum"`
	SalesDate    string  `json:"salesDate"`
	SalesDateIn  *string `json:"salesDateIn"`
	SalesDateOut *string `json:"salesDateOut"`

	BranchCode    string  `json:"branchCode"`
	BranchName    *string `json:"branchName"`
	ExtBranchCode *string `json:"extBranchCode"`

	MemberCode         *string `json:"memberCode"`
	MemberName         *string `json:"memberName"`
	ExternalMemberCode *string `json:"externalMemberCode"`

	TableID          *string    `json:"tableID"`
	TableName        *string    `json:"tableName"`
	VisitPurposeID   *string    `json:"visitPurposeID"`
	VisitPurposeName *string    `json:"visitPurposeName"`
	VisitorTypeID    *string    `json:"visitorTypeID"`
	VisitorTypeName  *string    `json:"visitorTypeName"`
	PaxTotal         flexNumber `json:"paxTotal"`

	Subtotal             *float64 `json:"subtotal"`
	DiscountTotal        *float64 `json:"discountTotal"`
	MenuDiscountTotal    *float64 `json:"menuDiscountTotal"`
	PromotionDiscount    *float64 `json:"promotionDiscount"`
	VoucherDiscountTotal *float64 `json:"voucherDiscountTotal"`
	OtherTaxTotal        *float64 `json:"otherTaxTotal"`
	VatTotal             *float64 `json:"vatTotal"`
	OtherVatTotal        *float64 `json:"otherVatTotal"`
	DeliveryCost         *float64 `json:"deliveryCost"`
	OrderFee             *float64 `json:"orderFee"`
	GrandTotal           *float64 `json:"grandTotal"`
	VoucherTotal         *float64 `json:"voucherTotal"`
	RoundingTotal        *float64 `json:"roundingTotal"`
	PaymentTotal         *float64 `json:"paymentTotal"`

	BillingPrintCount *int    `json:"billingPrintCount"`
	PaymentPrintCount *int    `json:"paymentPrintCount"`
	AdditionalInfo    *string `json:"additionalInfo"`
	PromotionID       *string `json:"promotionID"`
	PromotionName     *string `json:"promotionName"`
	FlagInclusive     *string `json:"flagInclusive"`
	StatusID          *string `json:"statusID"`
	StatusName        *string `json:"statusName"`
	FullName          *string `json:"fullName"`
	Email             *string `json:"email"`
	PhoneNumber       *string `json:"phoneNumber"`
	CreatedBy         *string `json:"createdBy"`
	EditedBy          *string `json:"editedBy"`
	EditedDate        *string `json:"editedDate"`

	ParentLinkSalesNum *string           `json:"parentLinkSalesNum"`
	ChildLinkSalesNum  []json.RawMessage `json:"childLinkSalesNum"`
	MergeTable         []json.RawMessage `json:"mergeTable"`

	SalesPayments []esbPayment  `json:"salesPayments"`
	SalesMenus    []esbMenuItem `json:"salesMenus"`
}

type esbPayment struct {
	SalesPaymentBackendID string   `json:"salesPaymentBackendID"`
	SalesPaymentPosID     *string  `json:"salesPaymentPosID"`
	PaymentMethodTypeID   *string  `json:"paymentMethodTypeID"`
	PaymentMethodTypeName *string  `json:"paymentMethodTypeName"`
	PaymentMethodID       *string  `json:"paymentMethodID"`
	PaymentMethodName     *string  `json:"paymentMethodName"`
	VoucherCode           *string  `json:"voucherCode"`
	CardNumber            *string  `json:"cardNumber"`
	BankName              *string  `json:"bankName"`
	AccountName           *string  `json:"accountName"`
	PaymentAmount         *float64 `json:"paymentAmount"`
	FullPaymentAmount     *float64 `json:"fullPaymentAmount"`
	// notes, selfOrderID, verificationCode exist on the ESB payload but are
	// dropped by the original transform -- not stored anywhere.
}

type esbMenuItem struct {
	SalesDate  *string `json:"salesDate"`
	BranchCode *string `json:"branchCode"`
	MenuID     string  `json:"menuID"`
	BatchID    *string `json:"batchID"`

	MenuCategoryID         *string `json:"menuCategoryID"`
	MenuCategoryName       *string `json:"menuCategoryName"`
	MenuCategoryDetailID   *string `json:"menuCategoryDetailID"`
	MenuCategoryDetailName *string `json:"menuCategoryDetailName"`
	MenuName               *string `json:"menuName"`
	MenuCode               *string `json:"menuCode"`

	Qty           *float64 `json:"qty"`
	OriginalPrice *float64 `json:"originalPrice"`
	Price         *float64 `json:"price"`
	Discount      *float64 `json:"discount"`
	DiscountValue *float64 `json:"discountValue"`
	OtherTaxValue *float64 `json:"otherTaxValue"`
	VatValue      *float64 `json:"vatValue"`
	Total         *float64 `json:"total"`

	Notes             *string `json:"notes"`
	StatusID          *string `json:"statusID"`
	StatusName        *string `json:"statusName"`
	PromotionDetailID *string `json:"promotionDetailID"`
	MenuPromotionID   *string `json:"menuPromotionID"`
	SalesType         *string `json:"salesType"`

	Packages []json.RawMessage `json:"packages"`
	Extras   []json.RawMessage `json:"extras"`
	// branchName, billNum, the item-level salesNum, otherTax, vat, otherVat,
	// otherVatValue exist on the ESB payload but are dropped by the original
	// transform -- not stored anywhere.
}

// paginationHeaders mirrors the x-pagination-* response headers ESB returns
// on the sales-information endpoint (there is no envelope in the body).
type paginationHeaders struct {
	TotalCount  int
	PageCount   int
	CurrentPage int
	PerPage     int
}
