package etl

import (
	"strings"
	"time"

	"github.com/Operations-ESB/dashboard-sales/api/internal/model"
)

// orNull mirrors the old transform's `(v) => v ? v : null` -- an empty
// string becomes nil; a nil pointer (JSON null/missing) stays nil.
func orNull(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// numPtr mirrors `(v) => typeof v === "number" ? v : 0` -- ESB money fields
// are stored as 0, never NULL, when missing.
func numPtr(f *float64) *float64 {
	v := 0.0
	if f != nil {
		v = *f
	}
	return &v
}

// normalizeMemberCode mirrors the old `normalizeMemberCode`: whitespace-only
// values are treated as absent, but the ORIGINAL (untrimmed) value is kept
// when present -- this is intentional in the original, not an oversight.
func normalizeMemberCode(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

// esbDateLayouts are tried in order when parsing ESB date/datetime strings.
// The original Edge Function never parsed these itself -- it passed the raw
// string to Postgres and let the column type coerce it. Go has no such
// implicit conversion, so this list should be checked against a real ESB
// response before the first live sync and adjusted if needed.
var esbDateLayouts = []string{
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseESBTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	for _, layout := range esbDateLayouts {
		if t, err := time.Parse(layout, *s); err == nil {
			return &t
		}
	}
	return nil
}

func mustParseESBDate(s string) time.Time {
	if t := parseESBTime(&s); t != nil {
		return *t
	}
	return time.Time{}
}

// TransformResult holds one day's records, deduplicated and mapped to our
// schema, ready for Upsert.
type TransformResult struct {
	Outlets  []model.Outlet
	Sales    []model.RawSale
	Payments []model.RawSalesPayment
	Items    []model.RawSalesMenuItem
}

// Transform ports supabase/functions/sync-esb/transform.ts exactly:
//   - outlets dedupe by branch_code, FIRST occurrence wins.
//   - sales/payments/menu items dedupe by their conflict key, LAST
//     occurrence wins (both to avoid a same-key double-write within one
//     upsert batch, same reason the original uses a Map).
//   - line_seq is the 0-based index of the item in salesMenus[], since ESB
//     provides no other reliable per-line key.
//   - one syncedAt timestamp is shared by every row in this call (one value
//     per sync-esb invocation for one day, not per record).
//
// NonSalesPaymentTypeID is the ESB paymentMethodTypeID of "non sales"
// payments. ESB does not allow it to be combined with other payment types on
// one bill, so any bill carrying it is a non sales bill as a whole.
const NonSalesPaymentTypeID = "7"

// isNonSales reports whether a record is paid with the non sales payment type.
func isNonSales(r esbSaleRecord) bool {
	for _, p := range r.SalesPayments {
		if p.PaymentMethodTypeID != nil && strings.TrimSpace(*p.PaymentMethodTypeID) == NonSalesPaymentTypeID {
			return true
		}
	}
	return false
}

func Transform(records []esbSaleRecord, syncedAt time.Time) TransformResult {
	outletsByCode := map[string]model.Outlet{}
	outletOrder := []string{}
	salesByNum := map[string]model.RawSale{}
	salesOrder := []string{}
	paymentsByKey := map[string]model.RawSalesPayment{}
	paymentOrder := []string{}
	itemsByKey := map[string]model.RawSalesMenuItem{}
	itemOrder := []string{}

	for _, r := range records {
		if _, seen := outletsByCode[r.BranchCode]; !seen {
			branchName := r.BranchCode
			if r.BranchName != nil {
				branchName = *r.BranchName
			}
			outletsByCode[r.BranchCode] = model.Outlet{
				BranchCode:    r.BranchCode,
				BranchName:    branchName,
				ExtBranchCode: orNull(r.ExtBranchCode),
			}
			outletOrder = append(outletOrder, r.BranchCode)
		}

		if _, seen := salesByNum[r.SalesNum]; !seen {
			salesOrder = append(salesOrder, r.SalesNum)
		}
		salesByNum[r.SalesNum] = model.RawSale{
			SalesNum:     r.SalesNum,
			BillNum:      orNull(r.BillNum),
			SalesDate:    mustParseESBDate(r.SalesDate),
			SalesDateIn:  parseESBTime(r.SalesDateIn),
			SalesDateOut: parseESBTime(r.SalesDateOut),

			BranchCode:    r.BranchCode,
			BranchName:    orNull(r.BranchName),
			ExtBranchCode: orNull(r.ExtBranchCode),

			MemberCode:         normalizeMemberCode(r.MemberCode),
			MemberName:         orNull(r.MemberName),
			ExternalMemberCode: orNull(r.ExternalMemberCode),

			TableID:          orNull(r.TableID),
			TableName:        orNull(r.TableName),
			VisitPurposeID:   orNull(r.VisitPurposeID),
			VisitPurposeName: orNull(r.VisitPurposeName),
			VisitorTypeID:    orNull(r.VisitorTypeID),
			VisitorTypeName:  orNull(r.VisitorTypeName),
			PaxTotal:         flexNumberToIntPtr(r.PaxTotal),

			Subtotal:             numPtr(r.Subtotal),
			DiscountTotal:        numPtr(r.DiscountTotal),
			MenuDiscountTotal:    numPtr(r.MenuDiscountTotal),
			PromotionDiscount:    numPtr(r.PromotionDiscount),
			VoucherDiscountTotal: numPtr(r.VoucherDiscountTotal),
			OtherTaxTotal:        numPtr(r.OtherTaxTotal),
			VatTotal:             numPtr(r.VatTotal),
			OtherVatTotal:        numPtr(r.OtherVatTotal),
			DeliveryCost:         numPtr(r.DeliveryCost),
			OrderFee:             numPtr(r.OrderFee),
			GrandTotal:           *numPtr(r.GrandTotal),
			VoucherTotal:         numPtr(r.VoucherTotal),
			RoundingTotal:        numPtr(r.RoundingTotal),
			PaymentTotal:         numPtr(r.PaymentTotal),

			BillingPrintCount: r.BillingPrintCount,
			PaymentPrintCount: r.PaymentPrintCount,
			AdditionalInfo:    orNull(r.AdditionalInfo),
			PromotionID:       orNull(r.PromotionID),
			PromotionName:     orNull(r.PromotionName),
			FlagInclusive:     orNull(r.FlagInclusive),
			StatusID:          orNull(r.StatusID),
			StatusName:        orNull(r.StatusName),
			IsNonSales:        isNonSales(r),
			FullName:          orNull(r.FullName),
			Email:             orNull(r.Email),
			PhoneNumber:       orNull(r.PhoneNumber),
			CreatedBy:         orNull(r.CreatedBy),
			EditedBy:          orNull(r.EditedBy),
			EditedDate:        parseESBTime(r.EditedDate),

			ParentLinkSalesNum: orNull(r.ParentLinkSalesNum),
			ChildLinkSalesNum:  rawJSONArray(r.ChildLinkSalesNum),
			MergeTable:         rawJSONArray(r.MergeTable),
			Raw:                mustMarshal(r),
			SyncedAt:           syncedAt,
		}

		for _, p := range r.SalesPayments {
			key := r.SalesNum + "|" + p.SalesPaymentBackendID
			if _, seen := paymentsByKey[key]; !seen {
				paymentOrder = append(paymentOrder, key)
			}
			paymentsByKey[key] = model.RawSalesPayment{
				SalesNum:              r.SalesNum,
				SalesPaymentBackendID: p.SalesPaymentBackendID,
				SalesPaymentPosID:     orNull(p.SalesPaymentPosID),
				PaymentMethodTypeID:   orNull(p.PaymentMethodTypeID),
				PaymentMethodTypeName: orNull(p.PaymentMethodTypeName),
				PaymentMethodID:       orNull(p.PaymentMethodID),
				PaymentMethodName:     orNull(p.PaymentMethodName),
				VoucherCode:           orNull(p.VoucherCode),
				CardNumber:            orNull(p.CardNumber),
				BankName:              orNull(p.BankName),
				AccountName:           orNull(p.AccountName),
				PaymentAmount:         numPtr(p.PaymentAmount),
				FullPaymentAmount:     numPtr(p.FullPaymentAmount),
			}
		}

		for lineSeq, m := range r.SalesMenus {
			key := r.SalesNum + "|" + itoa(lineSeq)
			if _, seen := itemsByKey[key]; !seen {
				itemOrder = append(itemOrder, key)
			}
			salesDate := r.SalesDate
			if m.SalesDate != nil {
				salesDate = *m.SalesDate
			}
			branchCode := r.BranchCode
			if m.BranchCode != nil {
				branchCode = *m.BranchCode
			}
			itemsByKey[key] = model.RawSalesMenuItem{
				SalesNum:   r.SalesNum,
				LineSeq:    lineSeq,
				MenuID:     m.MenuID,
				BatchID:    orNull(m.BatchID),
				SalesDate:  mustParseESBDate(salesDate),
				BranchCode: branchCode,

				MenuCategoryID:         orNull(m.MenuCategoryID),
				MenuCategoryName:       orNull(m.MenuCategoryName),
				MenuCategoryDetailID:   orNull(m.MenuCategoryDetailID),
				MenuCategoryDetailName: orNull(m.MenuCategoryDetailName),
				MenuName:               orNull(m.MenuName),
				MenuCode:               orNull(m.MenuCode),

				Qty:   *numPtr(m.Qty),
				Total: *numPtr(m.Total),
				// original_price/price/discount/discount_value/other_tax_value/
				// vat_value stay NULL when absent -- the original transform
				// uses `?? null` here, not num(), unlike qty/total.
				OriginalPrice: m.OriginalPrice,
				Price:         m.Price,
				Discount:      m.Discount,
				DiscountValue: m.DiscountValue,
				OtherTaxValue: m.OtherTaxValue,
				VatValue:      m.VatValue,

				Notes:             orNull(m.Notes),
				StatusID:          orNull(m.StatusID),
				StatusName:        orNull(m.StatusName),
				PromotionDetailID: orNull(m.PromotionDetailID),
				MenuPromotionID:   orNull(m.MenuPromotionID),
				SalesType:         orNull(m.SalesType),
				Packages:          rawJSONArray(m.Packages),
				Extras:            rawJSONArray(m.Extras),
			}
		}
	}

	result := TransformResult{}
	for _, code := range outletOrder {
		result.Outlets = append(result.Outlets, outletsByCode[code])
	}
	for _, num := range salesOrder {
		result.Sales = append(result.Sales, salesByNum[num])
	}
	for _, key := range paymentOrder {
		result.Payments = append(result.Payments, paymentsByKey[key])
	}
	for _, key := range itemOrder {
		result.Items = append(result.Items, itemsByKey[key])
	}
	return result
}

func flexNumberToIntPtr(f flexNumber) *int {
	if f.Value == nil {
		return nil
	}
	n := int(*f.Value)
	return &n
}
