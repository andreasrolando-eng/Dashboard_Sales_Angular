package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/xuri/excelize/v2"

	"github.com/Operations-ESB/dashboard-sales/api/internal/service"
)

const xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

// writeBillsXLSX streams the bill export as a real .xlsx workbook. Dates and
// amounts are typed cells (not text), so Excel sorts and sums them regardless
// of the viewer's locale -- the reason this is not a CSV, whose separators
// Indonesian-locale Excel reads differently.
func writeBillsXLSX(w http.ResponseWriter, filename string, rows []service.Bill, outletNames map[string]string) {
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Bill"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	dateFmt := "dd-mm-yyyy"
	date, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFmt})
	money, _ := f.NewStyle(&excelize.Style{NumFmt: 3}) // #,##0
	moneyBold, _ := f.NewStyle(&excelize.Style{NumFmt: 3, Font: &excelize.Font{Bold: true}})

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = sw.SetColWidth(1, 1, 22)
	_ = sw.SetColWidth(2, 2, 12)
	_ = sw.SetColWidth(3, 3, 12)
	_ = sw.SetColWidth(4, 4, 30)
	_ = sw.SetColWidth(5, 5, 16)

	header := []any{}
	for _, h := range []string{"No. bill", "Tanggal", "Kode outlet", "Outlet", "Grand total"} {
		header = append(header, excelize.Cell{StyleID: bold, Value: h})
	}
	if err := sw.SetRow("A1", header); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	var total float64
	for i, b := range rows {
		billNum := ""
		if b.BillNum != nil {
			billNum = *b.BillNum
		}
		total += b.GrandTotal
		cell, _ := excelize.CoordinatesToCellName(1, i+2)
		if err := sw.SetRow(cell, []any{
			billNum,
			excelize.Cell{StyleID: date, Value: b.SalesDate},
			b.BranchCode,
			outletNames[b.BranchCode],
			excelize.Cell{StyleID: money, Value: b.GrandTotal},
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	cell, _ := excelize.CoordinatesToCellName(1, len(rows)+2)
	if err := sw.SetRow(cell, []any{
		excelize.Cell{StyleID: bold, Value: fmt.Sprintf("Total (%d bill)", len(rows))},
		nil, nil, nil,
		excelize.Cell{StyleID: moneyBold, Value: total},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := sw.Flush(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", xlsxContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	if _, err := f.WriteTo(w); err != nil {
		// Headers are already sent; all we can do is log.
		log.Printf("bills export: writing xlsx: %v", err)
	}
}
