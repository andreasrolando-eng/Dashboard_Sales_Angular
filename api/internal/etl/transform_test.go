package etl

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTransform_MoneyFieldsDefaultToZeroNotNull(t *testing.T) {
	rec := esbSaleRecord{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01", GrandTotal: nil}
	result := Transform([]esbSaleRecord{rec}, time.Now())

	if len(result.Sales) != 1 {
		t.Fatalf("len(Sales) = %d, want 1", len(result.Sales))
	}
	s := result.Sales[0]
	if s.GrandTotal != 0 {
		t.Errorf("GrandTotal = %v, want 0 (missing money field defaults to 0, not null)", s.GrandTotal)
	}
	if s.Subtotal == nil || *s.Subtotal != 0 {
		t.Errorf("Subtotal = %v, want pointer to 0", s.Subtotal)
	}
}

func TestTransform_StringFieldsEmptyBecomesNull(t *testing.T) {
	empty := ""
	rec := esbSaleRecord{
		SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01",
		BranchName: &empty, // present but empty -- orNull() should turn this into nil
		BillNum:    nil,    // absent -- stays nil
	}
	result := Transform([]esbSaleRecord{rec}, time.Now())

	s := result.Sales[0]
	if s.BranchName != nil {
		t.Errorf("BranchName = %v, want nil (empty string -> null)", *s.BranchName)
	}
	if s.BillNum != nil {
		t.Errorf("BillNum = %v, want nil", *s.BillNum)
	}
}

func TestTransform_OutletBranchNameFallsBackToBranchCodeOnlyWhenNil(t *testing.T) {
	empty := ""
	recNilName := esbSaleRecord{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01", BranchName: nil}
	result := Transform([]esbSaleRecord{recNilName}, time.Now())
	if result.Outlets[0].BranchName != "BR01" {
		t.Errorf("outlet branch_name = %q, want %q (falls back to branch_code when branchName is nil)", result.Outlets[0].BranchName, "BR01")
	}

	// An explicitly empty (non-nil) branchName is NOT replaced -- matches
	// JS `??` semantics (only null/undefined triggers the fallback).
	recEmptyName := esbSaleRecord{SalesNum: "SN2", SalesDate: "2026-02-03", BranchCode: "BR02", BranchName: &empty}
	result2 := Transform([]esbSaleRecord{recEmptyName}, time.Now())
	if result2.Outlets[0].BranchName != "" {
		t.Errorf("outlet branch_name = %q, want empty string preserved (?? doesn't trigger on empty string)", result2.Outlets[0].BranchName)
	}
}

func TestTransform_MemberCodeNormalization(t *testing.T) {
	whitespace := "   "
	untrimmed := " M1 "
	recWhitespace := esbSaleRecord{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01", MemberCode: &whitespace}
	recUntrimmed := esbSaleRecord{SalesNum: "SN2", SalesDate: "2026-02-03", BranchCode: "BR01", MemberCode: &untrimmed}

	result := Transform([]esbSaleRecord{recWhitespace, recUntrimmed}, time.Now())

	if result.Sales[0].MemberCode != nil {
		t.Errorf("whitespace-only member_code = %v, want nil", *result.Sales[0].MemberCode)
	}
	if result.Sales[1].MemberCode == nil || *result.Sales[1].MemberCode != " M1 " {
		t.Errorf("member_code = %v, want the ORIGINAL untrimmed value %q preserved", result.Sales[1].MemberCode, " M1 ")
	}
}

func TestTransform_LineSeqIsZeroBasedArrayIndex(t *testing.T) {
	rec := esbSaleRecord{
		SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01",
		SalesMenus: []esbMenuItem{
			{MenuID: "M-A"}, {MenuID: "M-B"}, {MenuID: "M-C"},
		},
	}
	result := Transform([]esbSaleRecord{rec}, time.Now())
	if len(result.Items) != 3 {
		t.Fatalf("len(Items) = %d, want 3", len(result.Items))
	}
	for i, item := range result.Items {
		if item.LineSeq != i {
			t.Errorf("Items[%d].LineSeq = %d, want %d", i, item.LineSeq, i)
		}
	}
}

func TestTransform_MenuItemPriceFieldsStayNullButQtyAndTotalDefaultToZero(t *testing.T) {
	rec := esbSaleRecord{
		SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01",
		SalesMenus: []esbMenuItem{{MenuID: "M-A"}}, // qty, total, price, discount all absent
	}
	result := Transform([]esbSaleRecord{rec}, time.Now())
	item := result.Items[0]
	if item.Qty != 0 {
		t.Errorf("Qty = %v, want 0 (num() default)", item.Qty)
	}
	if item.Total != 0 {
		t.Errorf("Total = %v, want 0 (num() default)", item.Total)
	}
	if item.Price != nil {
		t.Errorf("Price = %v, want nil (?? null, not num())", *item.Price)
	}
	if item.OriginalPrice != nil || item.Discount != nil || item.DiscountValue != nil || item.OtherTaxValue != nil || item.VatValue != nil {
		t.Errorf("expected all non-qty/total price fields to stay nil, got Item = %+v", item)
	}
}

func TestTransform_DedupSalesLastOccurrenceWins(t *testing.T) {
	first := esbSaleRecord{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01", GrandTotal: f(100)}
	second := esbSaleRecord{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01", GrandTotal: f(200)}

	result := Transform([]esbSaleRecord{first, second}, time.Now())
	if len(result.Sales) != 1 {
		t.Fatalf("len(Sales) = %d, want 1 (deduped by sales_num)", len(result.Sales))
	}
	if result.Sales[0].GrandTotal != 200 {
		t.Errorf("GrandTotal = %v, want 200 (last occurrence should win)", result.Sales[0].GrandTotal)
	}
}

func TestTransform_DedupOutletsFirstOccurrenceWins(t *testing.T) {
	nameA, nameB := "Nama Lama", "Nama Baru"
	first := esbSaleRecord{SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01", BranchName: &nameA}
	second := esbSaleRecord{SalesNum: "SN2", SalesDate: "2026-02-03", BranchCode: "BR01", BranchName: &nameB}

	result := Transform([]esbSaleRecord{first, second}, time.Now())
	if len(result.Outlets) != 1 {
		t.Fatalf("len(Outlets) = %d, want 1", len(result.Outlets))
	}
	if result.Outlets[0].BranchName != "Nama Lama" {
		t.Errorf("outlet branch_name = %q, want %q (FIRST occurrence should win for outlets, unlike sales)", result.Outlets[0].BranchName, "Nama Lama")
	}
}

func TestTransform_DedupPaymentsAndMenuItemsByCompositeKey(t *testing.T) {
	rec := esbSaleRecord{
		SalesNum: "SN1", SalesDate: "2026-02-03", BranchCode: "BR01",
		SalesPayments: []esbPayment{
			{SalesPaymentBackendID: "P1", PaymentAmount: f(100)},
			{SalesPaymentBackendID: "P1", PaymentAmount: f(999)}, // same key, should overwrite
		},
	}
	result := Transform([]esbSaleRecord{rec}, time.Now())
	if len(result.Payments) != 1 {
		t.Fatalf("len(Payments) = %d, want 1 (deduped by sales_num|backend_id)", len(result.Payments))
	}
	if *result.Payments[0].PaymentAmount != 999 {
		t.Errorf("PaymentAmount = %v, want 999 (last occurrence wins)", *result.Payments[0].PaymentAmount)
	}
}

func TestFlexNumber_AcceptsStringOrNumber(t *testing.T) {
	var withString, withNumber esbSaleRecord
	if err := json.Unmarshal([]byte(`{"salesNum":"SN1","salesDate":"2026-02-03","branchCode":"BR01","paxTotal":"3"}`), &withString); err != nil {
		t.Fatalf("unmarshal string paxTotal: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"salesNum":"SN1","salesDate":"2026-02-03","branchCode":"BR01","paxTotal":3}`), &withNumber); err != nil {
		t.Fatalf("unmarshal numeric paxTotal: %v", err)
	}
	if withString.PaxTotal.Value == nil || *withString.PaxTotal.Value != 3 {
		t.Errorf("string paxTotal = %v, want 3", withString.PaxTotal.Value)
	}
	if withNumber.PaxTotal.Value == nil || *withNumber.PaxTotal.Value != 3 {
		t.Errorf("numeric paxTotal = %v, want 3", withNumber.PaxTotal.Value)
	}
}

func f(v float64) *float64 { return &v }
